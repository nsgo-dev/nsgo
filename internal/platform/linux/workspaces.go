//go:build linux && !android

package linux

import (
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A window on every workspace (desktop.SetWindowWorkspaces) is one on every virtual desktop of an X11 window manager that supports EWMH's _NET_WM_DESKTOP, which GDK asks for with gdk_x11_surface_move_to_desktop. GDK sends that request only where the window manager lists _NET_WM_DESKTOP in _NET_SUPPORTED, which gdk_x11_screen_supports_net_wm_hint reads, so the environment reports the same (CanSetWindowWorkspaces). A window manager takes the request only for a window it manages, and GDK deletes the property each time it maps one that is not sticky, so the request goes again each time the GtkWindow maps. It goes from the GtkWindow's own "map" signal, whose handlers run after GtkWindow's class handler has presented the toplevel and sent XMapWindow, so the window manager has the MapRequest first; a child's map, as the view's, comes before the toplevel is presented, and GdkSurface's mapped property changes before XMapWindow too. Back on one desktop the request names the current desktop itself, since gdk_x11_surface_move_to_current_desktop does nothing for a window GDK knows is on every one. GTK 4.18 deprecated these X11 functions with the rest of its X11 API, and they remain. Wayland has no protocol for it: there the window stays on the workspace the compositor put it on.

// allDesktops is the _NET_WM_DESKTOP of a window on every desktop.
const allDesktops = 0xFFFFFFFF

var (
	gdkX11DisplayGetScreen        func(display uintptr) uintptr
	gdkX11ScreenSupportsNetWMHint func(screen uintptr, property *byte) int32
	gdkX11ScreenGetCurrentDesktop func(screen uintptr) uint32
	gdkX11SurfaceMoveToDesktop    func(surface uintptr, desktop uint32)
	// bindX11Desktops binds GDK's X11 desktop functions, false where its GDK has none.
	bindX11Desktops = sync.OnceValue(func() bool {
		if bindX11Panels(); !x11PanelsReady {
			return false
		}
		for _, symbol := range []string{"gdk_x11_display_get_screen", "gdk_x11_screen_supports_net_wm_hint", "gdk_x11_screen_get_current_desktop", "gdk_x11_surface_move_to_desktop"} {
			if _, err := purego.Dlsym(gtkLib, symbol); err != nil {
				return false
			}
		}
		purego.RegisterLibFunc(&gdkX11DisplayGetScreen, gtkLib, "gdk_x11_display_get_screen")
		purego.RegisterLibFunc(&gdkX11ScreenSupportsNetWMHint, gtkLib, "gdk_x11_screen_supports_net_wm_hint")
		purego.RegisterLibFunc(&gdkX11ScreenGetCurrentDesktop, gtkLib, "gdk_x11_screen_get_current_desktop")
		purego.RegisterLibFunc(&gdkX11SurfaceMoveToDesktop, gtkLib, "gdk_x11_surface_move_to_desktop")
		return true
	})
)

// desktopScreen returns GDK's X11 screen where GDK draws the app's windows on X11, Xwayland included, and the window manager supports _NET_WM_DESKTOP, else 0.
func desktopScreen() uintptr {
	if !bindX11Desktops() {
		return 0
	}
	display := gdkDisplayGetDefault()
	if display == 0 || gTypeIsA(display, gdkX11DisplayGetType()) == 0 {
		return 0
	}
	screen := gdkX11DisplayGetScreen(display)
	if screen == 0 || gdkX11ScreenSupportsNetWMHint(screen, cstr("_NET_WM_DESKTOP")) == 0 {
		return 0
	}
	return screen
}

// canSetWorkspaces reports whether setWorkspaces reaches a window manager for w: a window of its own that one manages, not the app's own surface (Options.Hidden) or a tray icon's, which show in none, on X11 under a window manager that supports _NET_WM_DESKTOP.
func (w *window) canSetWorkspaces() bool {
	return w.win != 0 && !w.embedded && !w.unmanaged && !(w.primary && w.opts.Hidden) && w.kind != driver.SurfaceTrayIcon && desktopScreen() != 0
}

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
	if !all && !w.onAllDesktops || !w.canSetWorkspaces() || gtkWidgetGetMapped(w.win) == 0 {
		return
	}
	surface := gtkNativeGetSurface(w.win)
	if surface == 0 {
		return
	}
	desktop := uint32(allDesktops)
	if !all {
		desktop = gdkX11ScreenGetCurrentDesktop(desktopScreen())
	}
	gdkX11SurfaceMoveToDesktop(surface, desktop)
	w.onAllDesktops = all
}
