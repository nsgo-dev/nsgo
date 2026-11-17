//go:build linux && !android

package linux

import (
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// desktopEvent is Xlib's XClientMessageEvent padded to the size of the XEvent union XNextEvent writes, with Go laying out its fields as C does.
type desktopEvent struct {
	typ         int32
	serial      uintptr
	sendEvent   int32
	display     uintptr
	window      uintptr
	messageType uintptr
	format      int32
	data        [5]int
	_           [12]int
}

// desktopXlib is the Xlib the workspace tests stand in for a window manager with and listen as one through.
var desktopXlib struct {
	openDisplay    func(name *byte) uintptr
	closeDisplay   func(display uintptr) int32
	rootWindow     func(display uintptr) uintptr
	createWindow   func(display, parent uintptr, x, y int32, width, height, border uint32, borderPixel, background uintptr) uintptr
	destroyWindow  func(display, window uintptr) int32
	changeProperty func(display, window, property, typ uintptr, format, mode int32, data *int, n int32) int32
	deleteProperty func(display, window, property uintptr) int32
	getProperty    func(display, window, property uintptr, offset, length int, del int32, reqType uintptr, actualType *uintptr, actualFormat *int32, n, after *uint, data **byte) int32
	free           func(data *byte) int32
	selectInput    func(display, window uintptr, mask int) int32
	sync           func(display uintptr, discard int32) int32
	pending        func(display uintptr) int32
	nextEvent      func(display uintptr, event *desktopEvent) int32
	internAtom     func(display uintptr, name *byte, onlyIfExists int32) uintptr
}

// bindDesktopXlib binds desktopXlib, false without libX11.
var bindDesktopXlib = sync.OnceValue(func() bool {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return false
	}
	x := &desktopXlib
	purego.RegisterLibFunc(&x.openDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&x.closeDisplay, lib, "XCloseDisplay")
	purego.RegisterLibFunc(&x.rootWindow, lib, "XDefaultRootWindow")
	purego.RegisterLibFunc(&x.createWindow, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&x.destroyWindow, lib, "XDestroyWindow")
	purego.RegisterLibFunc(&x.changeProperty, lib, "XChangeProperty")
	purego.RegisterLibFunc(&x.deleteProperty, lib, "XDeleteProperty")
	purego.RegisterLibFunc(&x.getProperty, lib, "XGetWindowProperty")
	purego.RegisterLibFunc(&x.free, lib, "XFree")
	purego.RegisterLibFunc(&x.selectInput, lib, "XSelectInput")
	purego.RegisterLibFunc(&x.sync, lib, "XSync")
	purego.RegisterLibFunc(&x.pending, lib, "XPending")
	purego.RegisterLibFunc(&x.nextEvent, lib, "XNextEvent")
	purego.RegisterLibFunc(&x.internAtom, lib, "XInternAtom")
	return true
})

// The X11 constants the workspace tests use.
const (
	desktopXAAtom, desktopXACardinal, desktopXAWindow = 4, 6, 33
	desktopPropModeReplace                            = 0
	desktopSubstructureNotifyMask                     = 1 << 19
	desktopClientMessage                              = 33
)

// TestWindowWorkspacesAskTheWindowManager listens on the X11 root window, as a window manager hears it, for the _NET_WM_DESKTOP requests SetWindowWorkspaces sends: every desktop for WindowWorkspacesAll, again as the window maps after it hid, and the current desktop back on one. Where no window manager runs, as under Xvfb, the test stands in for one, first one that lists only _NET_CURRENT_DESKTOP in _NET_SUPPORTED, under which the environment says the workspaces cannot be set, then one that lists _NET_WM_DESKTOP too; GDK and the environment read the list. On Wayland, which GDK may choose with DISPLAY set too, the environment says the workspaces cannot be set.
func TestWindowWorkspacesAskTheWindowManager(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	if !bindDesktopXlib() {
		t.Skip("no libX11")
	}
	x := &desktopXlib
	// A connection of the test's own, which hears what is sent to the root window apart from GTK's.
	display := x.openDisplay(nil)
	if display == 0 {
		t.Skip("cannot open the X11 display")
	}
	defer x.closeDisplay(display)
	root := x.rootWindow(display)
	atom := func(name string) uintptr { return x.internAtom(display, cstr(name), 0) }
	check, supported, current := atom("_NET_SUPPORTING_WM_CHECK"), atom("_NET_SUPPORTED"), atom("_NET_CURRENT_DESKTOP")

	// Stand in for a window manager only where none runs: one that does keeps its own hints, and its desktops.
	var actualType uintptr
	var format int32
	var n, after uint
	var data *byte
	x.getProperty(display, root, check, 0, 1, 0, desktopXAWindow, &actualType, &format, &n, &after, &data)
	if data != nil {
		x.free(data)
	}
	if n != 0 {
		workspacesUnder(t, display, true, -1)
		return
	}
	for _, desktops := range []bool{false, true} {
		t.Run(map[bool]string{false: "without _NET_WM_DESKTOP", true: "with _NET_WM_DESKTOP"}[desktops], func(t *testing.T) {
			const fakeCurrent = 2
			wm := x.createWindow(display, root, 0, 0, 1, 1, 0, 0, 0)
			for _, w := range []uintptr{root, wm} {
				id := int(wm)
				x.changeProperty(display, w, check, desktopXAWindow, 32, desktopPropModeReplace, &id, 1)
			}
			hints := []int{int(current)}
			if desktops {
				hints = append(hints, int(atom("_NET_WM_DESKTOP")))
			}
			x.changeProperty(display, root, supported, desktopXAAtom, 32, desktopPropModeReplace, &hints[0], int32(len(hints)))
			desktop := fakeCurrent
			x.changeProperty(display, root, current, desktopXACardinal, 32, desktopPropModeReplace, &desktop, 1)
			x.sync(display, 0)
			defer func() {
				// GDK forgets the window manager as its check window goes.
				for _, p := range []uintptr{check, supported, current} {
					x.deleteProperty(display, root, p)
				}
				x.destroyWindow(display, wm)
				x.sync(display, 0)
			}()
			workspacesUnder(t, display, desktops, fakeCurrent)
		})
	}
}

// workspacesUnder opens a window under the window manager the X11 display has, which supports _NET_WM_DESKTOP if desktops, and checks the requests SetWindowWorkspaces sends for it: back on one, for the desktop wantCurrent, or where it is -1, as for a real window manager, for any but every desktop.
func workspacesUnder(t *testing.T, display uintptr, desktops bool, wantCurrent int) {
	x := &desktopXlib
	scopes := make(chan *core.Scope, 1)
	app := func(cx *core.Scope) core.Node {
		scopes <- cx
		return core.View{Style: core.Style{Background: driver.RGB(0xffffff)}}
	}
	w := startTestWindow(t, app, Options{Title: "nsgo-window-workspaces", Width: 200, Height: 100})
	cx := <-scopes
	var xid uintptr
	w.onLoop(t, func() { _, xid, _ = w.x11Window() })
	env := make(chan bool, 1)
	cx.Post(func() { env <- cx.Env().CanSetWindowWorkspaces() })
	can := <-env
	switch {
	case xid == 0:
		if can {
			t.Error("CanSetWindowWorkspaces is true off X11")
		}
		return
	case wantCurrent < 0 && !can:
		t.Skip("the window manager does not support _NET_WM_DESKTOP")
	case can != desktops:
		t.Fatalf("CanSetWindowWorkspaces is %v under a window manager that supports _NET_WM_DESKTOP: %v", can, desktops)
	case !can:
		return
	}

	wmDesktop := x.internAtom(display, cstr("_NET_WM_DESKTOP"), 0)
	x.selectInput(display, x.rootWindow(display), desktopSubstructureNotifyMask)
	x.sync(display, 0)
	// next returns the desktop and the source of the window's next _NET_WM_DESKTOP request: a CARDINAL of 32 bits, which Xlib sign-extends into the event's long.
	next := func(what string) (desktop uint32, source int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if x.pending(display) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			var ev desktopEvent
			x.nextEvent(display, &ev)
			if ev.typ == desktopClientMessage && ev.window == xid && ev.messageType == wmDesktop {
				if ev.format != 32 {
					t.Errorf("%s: format %d", what, ev.format)
				}
				return uint32(ev.data[0]), ev.data[1]
			}
		}
		t.Fatalf("%s: no _NET_WM_DESKTOP request", what)
		return 0, 0
	}
	expect := func(what string, want uint32) {
		t.Helper()
		if desktop, source := next(what); desktop != want || source != 1 {
			t.Errorf("%s: desktop %#x from source %d, want %#x from an application (1)", what, desktop, source, want)
		}
	}

	cx.Post(func() { cx.SetWindowWorkspaces(core.WindowWorkspacesAll) })
	expect("every workspace", allDesktops)
	w.onLoop(t, func() {
		gtkWidgetSetVisible(w.win, 0)
		gtkWidgetSetVisible(w.win, 1)
	})
	expect("shown again", allDesktops)
	cx.Post(func() { cx.SetWindowWorkspaces(core.WindowWorkspacesCurrent) })
	if wantCurrent >= 0 {
		expect("back on one", uint32(wantCurrent))
	} else if desktop, _ := next("back on one"); desktop == allDesktops {
		t.Error("back on one: the request is still for every desktop")
	}
}
