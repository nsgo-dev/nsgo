//go:build darwin && !ios

package macos

import (
	"context"
	"errors"
	"math"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
)

// QuickLookThumbnailPrincipalClass is the NSExtensionPrincipalClass declared
// in QuickLookThumbnail.appex's Info.plist and registered at run time by
// RunQuickLookThumbnail.
const QuickLookThumbnailPrincipalClass = "NSGoQuickLookThumbnailProvider"

// QuickLookThumbnailRequest is a file QuickLook asks a thumbnail of: its
// path and URL, the largest size in points the thumbnail may take and the
// pixels a point it is drawn at.
type QuickLookThumbnailRequest struct {
	Path, URL string
	Size      driver.Size
	Scale     float64
}

type quickLookThumbnailFactory func(QuickLookThumbnailRequest) (engine.App, engine.HostOptions)

const (
	// quickLookThumbnailTimeout bounds a thumbnail from its request to its
	// reply, waiting for a turn included: QuickLook gives up on slower
	// extensions, and Finder keeps the file's icon.
	quickLookThumbnailTimeout = 10 * time.Second
	// quickLookThumbnailRenders is how many thumbnails render at once;
	// Finder asks a folder's at the same time, from several threads.
	quickLookThumbnailRenders = 4
	// quickLookThumbnailSide bounds a thumbnail's side in points.
	quickLookThumbnailSide = 2048
)

var (
	quickLookThumbnailMakeApp      quickLookThumbnailFactory
	quickLookThumbnailOptions      Options
	quickLookThumbnailTimeoutAfter = quickLookThumbnailTimeout
	quickLookThumbnailTurns        = make(chan struct{}, quickLookThumbnailRenders)
)

// RunQuickLookThumbnail registers NSGoQuickLookThumbnailProvider (a
// QLThumbnailProvider) and hands the process to Foundation's
// NSExtensionMain. Each thumbnail mounts an embedded NSGo View in a window
// that never shows, at the request's largest size, and replies with what
// its layers draw.
func RunQuickLookThumbnail(makeApp func(QuickLookThumbnailRequest) (engine.App, engine.HostOptions), opts Options) {
	lib, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	registerQuickLookThumbnailProvider(makeApp, opts)
	var extensionMain func(int, unsafe.Pointer) int
	purego.RegisterLibFunc(&extensionMain, lib, "NSExtensionMain")
	argv := make([]*byte, 0, len(os.Args)+1)
	for _, arg := range os.Args {
		b := append([]byte(arg), 0)
		argv = append(argv, &b[0])
	}
	argv = append(argv, nil)
	code := extensionMain(len(os.Args), unsafe.Pointer(&argv[0]))
	runtime.KeepAlive(argv)
	os.Exit(code)
}

// registerQuickLookThumbnailProvider loads AppKit and QuickLookThumbnailing,
// stores the active thumbnail factory and options, and registers
// NSGoQuickLookThumbnailProvider.
func registerQuickLookThumbnailProvider(makeApp quickLookThumbnailFactory, opts Options) objc.Class {
	loadAppKit()
	_, _ = purego.Dlopen("/System/Library/Frameworks/QuickLookThumbnailing.framework/QuickLookThumbnailing", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	apple.RegisterFonts(opts.Fonts)
	quickLookThumbnailMakeApp = makeApp
	quickLookThumbnailOptions = opts
	return quickLookThumbnailProvider()
}

// quickLookThumbnailProvider registers NSGoQuickLookThumbnailProvider once.
var quickLookThumbnailProvider = sync.OnceValue(func() objc.Class {
	super := apple.Class("QLThumbnailProvider")
	if super == 0 {
		super = apple.Class("NSObject")
	}
	return register(QuickLookThumbnailPrincipalClass, super, nil, []objc.MethodDef{{
		Cmd: sel("provideThumbnailForFileRequest:completionHandler:"),
		Fn:  quickLookProvideThumbnail,
	}})
})

// quickLookProvideThumbnail answers a request, on whichever thread
// QuickLook sends it, without waiting: the thumbnail renders on the main
// thread and the copied handler gets the reply once it is drawn.
func quickLookProvideThumbnail(_ objc.ID, _ objc.SEL, request, handler objc.ID) {
	url := request.Send(sel("fileURL"))
	var size apple.CGSize
	var scale float64
	if request != 0 {
		size = objc.Send[apple.CGSize](request, sel("maximumSize"))
		scale = objc.Send[float64](request, sel("scale"))
	}
	provideQuickLookThumbnail(url, size, scale, handler)
}

// provideQuickLookThumbnail starts the thumbnail of url, at most size
// points at scale pixels each, which handler, copied, gets.
func provideQuickLookThumbnail(url objc.ID, size apple.CGSize, scale float64, handler objc.ID) {
	if handler == 0 {
		return
	}
	handler = handler.Send(sel("copy"))
	if url == 0 || quickLookThumbnailMakeApp == nil {
		replyQuickLookThumbnail(handler, 0, 1, errors.New("QuickLook thumbnail has no file URL"))
		return
	}
	req := QuickLookThumbnailRequest{
		Path:  apple.GoString(url.Send(sel("path"))),
		URL:   apple.GoString(url.Send(sel("absoluteString"))),
		Size:  driver.Size{Width: driver.Points(min(size.Width, quickLookThumbnailSide)), Height: driver.Points(min(size.Height, quickLookThumbnailSide))},
		Scale: scale,
	}
	if !(req.Scale > 0) || math.IsInf(req.Scale, 0) {
		req.Scale = 1
	}
	if req.Path == "" || !(req.Size.Width >= 1 && req.Size.Height >= 1) {
		replyQuickLookThumbnail(handler, 0, 1, errors.New("QuickLook thumbnail request has no file path or size"))
		return
	}
	url.Send(sel("retain"))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), quickLookThumbnailTimeoutAfter)
		defer cancel()
		var scoped bool
		apple.WithPool(func() { scoped = objc.Send[bool](url, sel("startAccessingSecurityScopedResource")) })
		image, err := renderQuickLookThumbnail(ctx, req)
		apple.WithPool(func() {
			if scoped {
				url.Send(sel("stopAccessingSecurityScopedResource"))
			}
			url.Send(selRelease)
			replyQuickLookThumbnail(handler, image, req.Scale, err)
		})
	}()
}

// renderQuickLookThumbnail draws req's thumbnail into a CGImage the caller
// owns, waiting for a turn among the renders and for the app's commits
// until ctx ends.
func renderQuickLookThumbnail(ctx context.Context, req QuickLookThumbnailRequest) (uintptr, error) {
	select {
	case quickLookThumbnailTurns <- struct{}{}:
		defer func() { <-quickLookThumbnailTurns }()
	case <-ctx.Done():
		return 0, errors.New("QuickLook thumbnail timed out waiting for its turn")
	}
	app, hostOpts := quickLookThumbnailMakeApp(req)
	var t *quickLookThumbnail
	if !onMainWithin(ctx, func() { t = newQuickLookThumbnail(app, hostOpts, req) }) {
		// The view made after the deadline is closed as it is made.
		apple.OnMain(func() {
			if t != nil {
				t.close()
			}
		})
		return 0, errors.New("QuickLook thumbnail timed out starting")
	}
	defer apple.OnMain(t.close)
	if t.view.s.engine == nil {
		return 0, errors.New("QuickLook thumbnail's app did not start")
	}
	// The app settles once a round of its engine's work commits nothing
	// more: a few rounds let what it posts as it mounts, such as a file it
	// reads, show; a deadline cuts an app that never settles.
	applied := uint64(0)
	for round := 0; round < 20; round++ {
		if !syncWithin(ctx, t.view.s.engine) {
			return 0, errors.New("QuickLook thumbnail timed out rendering")
		}
		var batch uint64
		if !onMainWithin(ctx, func() { batch = t.flush() }) {
			return 0, errors.New("QuickLook thumbnail timed out rendering")
		}
		if round > 0 && batch == applied {
			break
		}
		applied = batch
		select {
		case <-time.After(16 * time.Millisecond):
		case <-ctx.Done():
			return 0, errors.New("QuickLook thumbnail timed out rendering")
		}
	}
	var image uintptr
	if !onMainWithin(ctx, func() { image = t.draw() }) {
		apple.OnMain(func() { apple.Release(image) })
		return 0, errors.New("QuickLook thumbnail timed out drawing")
	}
	if image == 0 {
		return 0, errors.New("QuickLook thumbnail drew nothing")
	}
	return image, nil
}

// onMainWithin runs fn on the main thread and reports whether it ran
// before ctx ended; fn may still run later.
func onMainWithin(ctx context.Context, fn func()) bool {
	done := make(chan struct{})
	apple.OnMain(func() {
		if ctx.Err() == nil {
			fn()
		}
		close(done)
	})
	select {
	case <-done:
		return ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
}

// syncWithin waits for e's queued work to commit, or for ctx to end.
func syncWithin(ctx context.Context, e *engine.Engine) bool {
	done := make(chan struct{})
	go func() {
		e.Sync()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// quickLookThumbnail is a thumbnail's view in a window of its own that
// never shows, which the engine needs to start.
type quickLookThumbnail struct {
	window objc.ID
	view   *View
	size   driver.Size
	scale  float64
}

func newQuickLookThumbnail(app engine.App, hostOpts engine.HostOptions, req QuickLookThumbnailRequest) *quickLookThumbnail {
	opts := quickLookThumbnailOptions
	opts.Width, opts.Height = req.Size.Width, req.Size.Height
	opts.Host = hostOpts
	w, h := float64(req.Size.Width), float64(req.Size.Height)
	nsApp()
	const borderless, buffered = 0, 2
	window := objc.ID(apple.Class("NSWindow")).Send(selAlloc).Send(sel("initWithContentRect:styleMask:backing:defer:"),
		apple.Rect(0, 0, w, h), uint(borderless), uint(buffered), false)
	window.Send(sel("setReleasedWhenClosed:"), false)
	window.Send(selSetOpaque, false)
	window.Send(selSetBackgroundColor, objc.ID(apple.Class("NSColor")).Send(selClearColor))
	v := NewView(app, opts)
	native := objc.ID(v.Handle())
	native.Send(selSetFrame, apple.Rect(0, 0, w, h))
	// Moving into the window starts the app's engine at the view's size.
	window.Send(sel("contentView")).Send(selAddSubview, native)
	return &quickLookThumbnail{window: window, view: v, size: req.Size, scale: req.Scale}
}

// flush applies the batches the engine sent and has AppKit and Core
// Animation draw them, returning how many batches the view applied.
func (t *quickLookThumbnail) flush() uint64 {
	s := t.view.s
	s.applyBatches(0, true)
	s.commitPending()
	t.window.Send(sel("displayIfNeeded"))
	objc.ID(apple.Class("CATransaction")).Send(sel("flush"))
	return s.batch
}

// draw draws the view's layers into a CGImage the caller owns.
func (t *quickLookThumbnail) draw() uintptr {
	t.flush()
	return apple.LayerImage(t.view.s.native.Send(sel("layer")), float64(t.size.Width), float64(t.size.Height), t.scale)
}

func (t *quickLookThumbnail) close() {
	t.view.Close()
	t.window.Send(sel("close"))
	t.window.Send(selRelease)
}

// replyQuickLookThumbnail hands the copied handler a reply drawing image,
// of scale pixels a point, whose ownership it takes, or err, after which
// QuickLook keeps the file's icon, and releases handler.
func replyQuickLookThumbnail(handler objc.ID, image uintptr, scale float64, err error) {
	defer handler.Send(selRelease)
	if err != nil || image == 0 {
		if err == nil {
			err = errors.New("QuickLook thumbnail drew nothing")
		}
		apple.CallBlock(handler, 0, uintptr(quickLookNSError(err.Error())))
		return
	}
	w, h := apple.ImageSize(image)
	// The reply's context is in points, scale pixels each; the image's
	// pixels fill it.
	size := apple.CGSize{Width: float64(w) / scale, Height: float64(h) / scale}
	drawing := objcblock.NewWithCleanup(func(_ objc.Block, ctx uintptr) bool {
		apple.DrawImage(ctx, apple.Rect(0, 0, size.Width, size.Height), image)
		return true
	}, func() { apple.Release(image) })
	reply := objc.ID(apple.Class("QLThumbnailReply")).Send(sel("replyWithContextSize:drawingBlock:"), size, drawing)
	drawing.Release()
	if reply == 0 {
		apple.CallBlock(handler, 0, uintptr(quickLookNSError("QuickLook thumbnail has no reply")))
		return
	}
	apple.CallBlock(handler, uintptr(reply), 0)
}
