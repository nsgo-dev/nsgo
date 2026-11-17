//go:build linux && !android

package linux

import (
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A window on every workspace (desktop.SetWindowWorkspaces) is one on every virtual desktop of an X11 window manager that supports EWMH's _NET_WM_DESKTOP, which GDK asks for with gdk_x11_surface_move_to_desktop. A window manager takes the request only for a window it manages, and GDK deletes the property each time it maps one that is not sticky, so the request goes again each time the GtkWindow maps. It goes from the GtkWindow's own "map" signal, whose handlers run after GtkWindow's class handler has presented the toplevel and sent XMapWindow, so the window manager has the MapRequest first; a child's map, as the view's, comes before the toplevel is presented, and GdkSurface's mapped property changes before XMapWindow too. Back on one desktop the request names the current desktop itself, since gdk_x11_surface_move_to_current_desktop does nothing for a window GDK knows is on every one. Wayland has no protocol for it: there the window stays on the workspace the compositor put it on.

// allDesktops is the _NET_WM_DESKTOP of a window on every desktop.
const allDesktops = 0xFFFFFFFF

var (
	gdkX11DisplayType             func() uintptr
	gdkX11DisplayGetScreen        func(display uintptr) uintptr
	gdkX11ScreenGetCurrentDesktop func(screen uintptr) uint32
	gdkX11SurfaceMoveToDesktop    func(surface uintptr, desktop uint32)
	// bindX11Desktops binds GDK's X11 desktop functions, false where its GDK has none.
	bindX11Desktops = sync.OnceValue(func() bool {
		for _, symbol := range []string{"gdk_x11_display_get_type", "gdk_x11_display_get_screen", "gdk_x11_screen_get_current_desktop", "gdk_x11_surface_move_to_desktop"} {
			if _, err := purego.Dlsym(gtkLib, symbol); err != nil {
				return false
			}
		}
		purego.RegisterLibFunc(&gdkX11DisplayType, gtkLib, "gdk_x11_display_get_type")
		purego.RegisterLibFunc(&gdkX11DisplayGetScreen, gtkLib, "gdk_x11_display_get_screen")
		purego.RegisterLibFunc(&gdkX11ScreenGetCurrentDesktop, gtkLib, "gdk_x11_screen_get_current_desktop")
		purego.RegisterLibFunc(&gdkX11SurfaceMoveToDesktop, gtkLib, "gdk_x11_surface_move_to_desktop")
		return true
	})
)

// windowMappedCallback asks again for the workspaces the window manager forgot as the window unmapped, once the toplevel is mapped.
func windowMappedCallback(widget, data uintptr) {
	if w := windowOf(data); w != nil && !w.closing {
		w.applyWorkspaces()
	}
}

// setWorkspaces puts the window on every virtual desktop or back on the current one.
func (w *window) setWorkspaces(ws driver.WindowWorkspaces) {
	w.workspaces = ws
	w.applyWorkspaces()
}

// applyWorkspaces asks the window manager to put a mapped window on every desktop while its workspaces are all of them, and on the current one once they no longer are; a window never put on every desktop is left where it is.
func (w *window) applyWorkspaces() {
	all := w.workspaces == driver.WindowWorkspacesAll
	if w.win == 0 || !all && !w.onAllDesktops || gtkWidgetGetMapped(w.win) == 0 || !bindX11Desktops() {
		return
	}
	display := gdkDisplayGetDefault()
	if display == 0 || gTypeIsA(display, gdkX11DisplayType()) == 0 {
		return
	}
	surface := gtkNativeGetSurface(w.win)
	if surface == 0 {
		return
	}
	desktop := uint32(allDesktops)
	if !all {
		desktop = gdkX11ScreenGetCurrentDesktop(gdkX11DisplayGetScreen(display))
	}
	gdkX11SurfaceMoveToDesktop(surface, desktop)
	w.onAllDesktops = all
}
