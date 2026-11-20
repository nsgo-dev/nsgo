//go:build ios

package ios

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Scanning a code: an AVCaptureSession of the back camera whose AVCaptureMetadataOutput reads the kinds of code asked for, shown full screen in its AVCaptureVideoPreviewLayer under a navigation bar with the system's Cancel button, which UIKit words in the user's language.

const avFoundationPath = "/System/Library/Frameworks/AVFoundation.framework/AVFoundation"

const (
	// UIBarButtonSystemItemCancel, UIModalPresentationFullScreen and UIUserInterfaceStyleDark.
	barButtonCancel        = 1
	presentationFullScreen = 0
	interfaceStyleDark     = 2
)

var (
	scanOnce            sync.Once
	classScanController objc.Class
	classScanDelegate   objc.Class
	scans               = map[objc.ID]*scan{}
)

// scan is a scanner on screen: the request it answers, the session reading codes and the navigation controller showing it.
type scan struct {
	s       *surface
	id      uint32
	r       driver.ScanRequest
	session objc.ID
	output  objc.ID
	nav     objc.ID
	// started closes once startRunning returned, so stopRunning never comes first and leaves the camera on after a quick Cancel.
	started chan struct{}
}

// scanCode has the user allow the camera, then shows the scanner, or answers with why it cannot (withCamera).
func (s *surface) scanCode(r driver.ScanRequest, id uint32) {
	fail := func(why string) { s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id, Text: why}) }
	withCamera("scanning a code", fail, func() {
		if why := s.presentScanner(r, id); why != "" {
			fail(why)
		}
	})
}

// presentScanner starts a session of the back camera reading r's codes and presents it, returning why it could not.
func (s *surface) presentScanner(r driver.ScanRequest, id uint32) string {
	scanOnce.Do(registerScanner)
	if classScanController == 0 {
		return "AVFoundation is missing"
	}
	device := objc.ID(apple.Class("AVCaptureDevice")).Send(sel("defaultDeviceWithMediaType:"), apple.NSString("vide"))
	if device == 0 {
		return "this device has no camera"
	}
	input := objc.ID(apple.Class("AVCaptureDeviceInput")).Send(sel("deviceInputWithDevice:error:"), device, objc.ID(0))
	session := objc.ID(apple.Class("AVCaptureSession")).Send(selNew)
	if input == 0 || !objc.Send[bool](session, sel("canAddInput:"), input) {
		session.Send(selRelease)
		return "the camera cannot be used"
	}
	session.Send(sel("addInput:"), input)
	output := objc.ID(apple.Class("AVCaptureMetadataOutput")).Send(selNew)
	if !objc.Send[bool](session, sel("canAddOutput:"), output) {
		output.Send(selRelease)
		session.Send(selRelease)
		return "the camera cannot read codes"
	}
	session.Send(sel("addOutput:"), output)
	// The types are set once the output is in the session, which says which it reads.
	available := map[string]bool{}
	apple.ForEach(output.Send(sel("availableMetadataObjectTypes")), func(t objc.ID) { available[apple.GoString(t)] = true })
	types := objc.ID(apple.Class("NSMutableArray")).Send(sel("array"))
	for _, t := range apple.MetadataTypes(r) {
		if available[t] {
			types.Send(sel("addObject:"), apple.NSString(t))
		}
	}
	if objc.Send[uint](types, sel("count")) == 0 {
		output.Send(selRelease)
		session.Send(selRelease)
		return "the camera reads none of the codes asked for"
	}
	output.Send(sel("setMetadataObjectTypes:"), types)
	delegate := objc.ID(classScanDelegate).Send(selNew)
	output.Send(sel("setMetadataObjectsDelegate:queue:"), delegate, apple.MainQueue())

	controller := objc.ID(classScanController).Send(selNew)
	defer controller.Send(selRelease)
	view := controller.Send(sel("view"))
	view.Send(sel("setBackgroundColor:"), objc.ID(apple.Class("UIColor")).Send(sel("blackColor")))
	preview := objc.ID(apple.Class("AVCaptureVideoPreviewLayer")).Send(sel("layerWithSession:"), session)
	preview.Send(sel("setVideoGravity:"), apple.NSString("AVLayerVideoGravityResizeAspectFill"))
	view.Send(sel("layer")).Send(sel("addSublayer:"), preview)
	cancel := objc.ID(apple.Class("UIBarButtonItem")).Send(selAlloc).Send(sel("initWithBarButtonSystemItem:target:action:"), barButtonCancel, delegate, sel("cancel:"))
	controller.Send(sel("navigationItem")).Send(sel("setLeftBarButtonItem:"), cancel)
	cancel.Send(selRelease)
	nav := objc.ID(apple.Class("UINavigationController")).Send(selAlloc).Send(sel("initWithRootViewController:"), controller)
	nav.Send(sel("setModalPresentationStyle:"), presentationFullScreen)
	// The bar sits over the camera's picture: dark, so its button reads in either appearance.
	nav.Send(sel("setOverrideUserInterfaceStyle:"), interfaceStyleDark)
	started := make(chan struct{})
	scans[delegate] = &scan{s: s, id: id, r: r, session: session, output: output, nav: nav, started: started}
	s.topController().Send(sel("presentViewController:animated:completion:"), nav, true, objc.ID(0))
	// startRunning blocks until the camera runs, which Apple asks to keep off the main thread.
	go func() {
		session.Send(sel("startRunning"))
		close(started)
	}()
	return ""
}

// registerScanner registers the scanner's view controller, which keeps the camera's picture filling its view, and its delegate, which hears the codes read and the Cancel button.
func registerScanner() {
	if _, err := purego.Dlopen(avFoundationPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil || apple.Class("AVCaptureMetadataOutput") == 0 {
		return
	}
	classScanController = register("NsgoScanController", apple.Class("UIViewController"), nil, []objc.MethodDef{{
		Cmd: sel("viewDidLayoutSubviews"),
		Fn: func(self objc.ID, _ objc.SEL) {
			view := self.Send(sel("view"))
			bounds := objc.Send[apple.CGRect](view, selBounds)
			apple.ForEach(view.Send(sel("layer")).Send(sel("sublayers")), func(l objc.ID) { l.Send(selSetFrame, bounds) })
		},
	}})
	var protocols []*objc.Protocol
	if p := objc.GetProtocol("AVCaptureMetadataOutputObjectsDelegate"); p != nil {
		protocols = append(protocols, p)
	}
	classScanDelegate = register("NsgoScanDelegate", apple.Class("NSObject"), protocols, []objc.MethodDef{{
		Cmd: sel("captureOutput:didOutputMetadataObjects:fromConnection:"),
		Fn: func(self objc.ID, _ objc.SEL, _, objects, _ objc.ID) {
			sc := scans[self]
			if sc == nil {
				return
			}
			var code *driver.ScannedCode
			apple.ForEach(objects, func(o objc.ID) {
				if code != nil || !objc.Send[bool](o, sel("respondsToSelector:"), sel("stringValue")) {
					return
				}
				kind, ok := apple.MetadataKind(apple.GoString(o.Send(sel("type"))))
				text := o.Send(sel("stringValue"))
				// A code of binary data has no text to give.
				if ok && sc.r.Wants(kind) && text != 0 {
					code = &driver.ScannedCode{Kind: kind, Text: apple.GoString(text)}
				}
			})
			if code != nil {
				endScan(self, code)
			}
		},
	}, {
		Cmd: sel("cancel:"),
		Fn:  func(self objc.ID, _ objc.SEL, _ objc.ID) { endScan(self, nil) },
	}})
}

// endScan stops the scanner of delegate, dismisses it and answers its request with code, nil for a cancel.
func endScan(delegate objc.ID, code *driver.ScannedCode) {
	sc := scans[delegate]
	if sc == nil {
		return
	}
	delete(scans, delegate)
	// No more codes come to the delegate, which goes now.
	sc.output.Send(sel("setMetadataObjectsDelegate:queue:"), objc.ID(0), objc.ID(0))
	sc.output.Send(selRelease)
	session, nav, started := sc.session, sc.nav, sc.started
	go func() {
		<-started
		session.Send(sel("stopRunning"))
		session.Send(selRelease)
	}()
	nav.Send(sel("dismissViewControllerAnimated:completion:"), true, objc.ID(0))
	nav.Send(selRelease)
	delegate.Send(selRelease)
	ev := driver.Event{Kind: driver.EventPick, Count: sc.id}
	if code != nil {
		ev.Data = code
	}
	sc.s.engine.Dispatch(ev)
}
