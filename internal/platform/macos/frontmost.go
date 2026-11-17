//go:build darwin && !ios

package macos

import (
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

var (
	// frontmost is the application in front as the last activation or change of Space left it, which every surface's environment reports. The main thread alone reads and writes it.
	frontmost      driver.FrontmostAppInfo
	frontmostWatch sync.Once
)

// frontmostApp returns the application in front, following it from the first call on: NSWorkspace says when another app activates, and when the active Space changes, as it does when an app enters or leaves full screen. Main thread.
func frontmostApp() driver.FrontmostAppInfo {
	frontmostWatch.Do(func() {
		frontmost = readFrontmost()
		center := objc.ID(apple.Class("NSWorkspace")).Send(sel("sharedWorkspace")).Send(sel("notificationCenter"))
		for _, name := range []string{"NSWorkspaceDidActivateApplicationNotification", "NSWorkspaceActiveSpaceDidChangeNotification"} {
			apple.Observe(center, apple.Global(appKitPath, name), func(objc.ID) {
				if app := readFrontmost(); app != frontmost {
					frontmost = app
					for s := range surfaces {
						s.refresh()
					}
				}
			})
		}
	})
	return frontmost
}

// readFrontmost asks NSWorkspace for the application in front, and the window server whether its frontmost window covers a display. An app without a bundle identifier is known by its executable's path.
func readFrontmost() (app driver.FrontmostAppInfo) {
	front := objc.ID(apple.Class("NSWorkspace")).Send(sel("sharedWorkspace")).Send(sel("frontmostApplication"))
	if front == 0 {
		return app
	}
	app.ID = apple.GoString(front.Send(sel("bundleIdentifier")))
	if app.ID == "" {
		if url := front.Send(sel("executableURL")); url != 0 {
			app.ID = apple.GoString(url.Send(sel("path")))
		}
	}
	app.Name = apple.GoString(front.Send(sel("localizedName")))
	app.PID = int(int32(apple.Msg(front, sel("processIdentifier"))))
	app.FullScreen = app.PID > 0 && frontWindowCoversDisplay(int32(app.PID))
	return app
}

// frontWindowCoversDisplay reports whether the frontmost normal window of process pid on screen covers a whole display, menu bar included, as a window in full screen does and a zoomed one does not. The window server lists windows front to back, and their bounds need no permission.
func frontWindowCoversDisplay(pid int32) bool {
	if !loadWindowList() {
		return false
	}
	list := cgWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly|kCGWindowListExcludeDesktopElements, 0)
	if list == 0 {
		return false
	}
	defer cfRelease(list)
	kPID, kLayer, kBounds := apple.NSString("kCGWindowOwnerPID"), apple.NSString("kCGWindowLayer"), apple.NSString("kCGWindowBounds")
	number := func(d, key objc.ID) (float64, bool) {
		v := objc.ID(apple.Msg(d, sel("objectForKey:"), uintptr(key)))
		if v == 0 {
			return 0, false
		}
		return msgSendFloat(v, sel("doubleValue")), true
	}
	for i := range apple.Msg(list, sel("count")) {
		w := objc.ID(apple.Msg(list, sel("objectAtIndex:"), i))
		if owner, ok := number(w, kPID); !ok || int32(owner) != pid {
			continue
		}
		if layer, _ := number(w, kLayer); layer != 0 {
			continue
		}
		b := objc.ID(apple.Msg(w, sel("objectForKey:"), uintptr(kBounds)))
		if b == 0 {
			continue
		}
		x, _ := number(b, apple.NSString("X"))
		y, _ := number(b, apple.NSString("Y"))
		width, _ := number(b, apple.NSString("Width"))
		height, _ := number(b, apple.NSString("Height"))
		return coversDisplay(guideRect{x, y, width, height}, windowServerScreens())
	}
	return false
}

// windowServerScreens are the displays' frames in the window server's coordinates: from the main display's top left, y down.
func windowServerScreens() []guideRect {
	screens := objc.ID(apple.Class("NSScreen")).Send(sel("screens"))
	n := objc.Send[uint](screens, sel("count"))
	if n == 0 {
		return nil
	}
	mainHeight := objc.Send[apple.CGRect](screens.Send(sel("objectAtIndex:"), uint(0)), sel("frame")).Size.Height
	out := make([]guideRect, 0, n)
	for i := range n {
		f := objc.Send[apple.CGRect](screens.Send(sel("objectAtIndex:"), i), sel("frame"))
		out = append(out, guideRect{f.Origin.X, mainHeight - f.Origin.Y - f.Size.Height, f.Size.Width, f.Size.Height})
	}
	return out
}

// coversDisplay reports whether window covers one of screens whole, to within a point.
func coversDisplay(window guideRect, screens []guideRect) bool {
	const slack = 1
	for _, s := range screens {
		if s.W > 0 && s.H > 0 && window.X <= s.X+slack && window.Y <= s.Y+slack &&
			window.maxX() >= s.maxX()-slack && window.maxY() >= s.maxY()-slack {
			return true
		}
	}
	return false
}
