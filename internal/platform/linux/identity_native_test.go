//go:build linux && !android

package linux

import (
	"os"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestTheWindowClassIsTheAppID opens an app's window with an ID and reads the WM_CLASS X11 gives it: both parts are the ID, which the app's desktop entry names as its StartupWMClass, so a dock matches the running window to the entry. Wayland's app ID is the same program name, which no client can read back.
func TestTheWindowClassIsTheAppID(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") == "" {
		t.Skip("reads the window's class on an X11 display")
	}
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skip("no libX11")
	}
	var (
		realize      func(widget uintptr)
		getXDisplay  func(display uintptr) uintptr
		getXID       func(surface uintptr) uintptr
		getClassHint func(display, window uintptr, hint unsafe.Pointer) int32
		xFree        func(p uintptr) int32
	)
	purego.RegisterLibFunc(&realize, gtkLib, "gtk_widget_realize")
	purego.RegisterLibFunc(&getXDisplay, gtkLib, "gdk_x11_display_get_xdisplay")
	purego.RegisterLibFunc(&getXID, gtkLib, "gdk_x11_surface_get_xid")
	purego.RegisterLibFunc(&getClassHint, lib, "XGetClassHint")
	purego.RegisterLibFunc(&xFree, lib, "XFree")
	// The ID names the app's storage too; keep it out of the user's.
	for _, dir := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(dir, t.TempDir())
	}

	const id = "dev.nsgo.window-class-test"
	app := func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Background: driver.RGB(0xffffff)}}
	}
	w := startTestWindow(t, app, Options{Title: "nsgo-window-class", Width: 200, Height: 100, ID: id})
	var name, class string
	w.onLoop(t, func() {
		realize(w.win)
		surface := gtkNativeGetSurface(w.win)
		if surface == 0 {
			t.Error("the window has no surface")
			return
		}
		var hint struct{ name, class uintptr }
		if getClassHint(getXDisplay(gdkDisplayGetDefault()), getXID(surface), unsafe.Pointer(&hint)) == 0 {
			t.Error("the window has no WM_CLASS")
			return
		}
		name, class = goString(hint.name), goString(hint.class)
		xFree(hint.name)
		xFree(hint.class)
	})
	if name != id || class != id {
		t.Errorf("WM_CLASS is %q, %q, want %q for both", name, class, id)
	}
}
