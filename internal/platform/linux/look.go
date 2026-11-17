//go:build linux && !android

package linux

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// GTK 4 draws a window with an alpha channel wherever the display composites, an RGBA visual on X11 and any surface on Wayland, so a transparent window is an undecorated one whose theme background is gone: it loses the style class that paints it, and a style of NSGo's own clears it and the shadow GTK draws. NoShadow clears the shadow GTK draws around its own decorations; a window manager's, as on X11 with server-side ones, stays. GTK 4 keeps a window off the task list only on X11, through its GdkX11Surface's hint, and Wayland has no such hint.

// lookCSS clears the theme's background and shadow from windows of the classes NSGo gives them.
const lookCSS = `
window.nsgo-transparent { background: none; box-shadow: none; }
window.nsgo-no-shadow, window.nsgo-no-shadow.csd { box-shadow: none; }
`

var (
	gtkWidgetAddCSSClass               func(widget uintptr, class *byte)
	gtkWidgetRemoveCSSClass            func(widget uintptr, class *byte)
	gdkX11SurfaceSetSkipTaskbarHint    func(surface uintptr, skip int32)
	gdkX11SurfaceSetSkipTaskbarHintSet bool
	// bindLook binds what windows' looks need and gives the display lookCSS, above the theme, once.
	bindLook = sync.OnceFunc(func() {
		purego.RegisterLibFunc(&gtkWidgetAddCSSClass, gtkLib, "gtk_widget_add_css_class")
		purego.RegisterLibFunc(&gtkWidgetRemoveCSSClass, gtkLib, "gtk_widget_remove_css_class")
		if _, err := purego.Dlsym(gtkLib, "gdk_x11_surface_set_skip_taskbar_hint"); err == nil {
			purego.RegisterLibFunc(&gdkX11SurfaceSetSkipTaskbarHint, gtkLib, "gdk_x11_surface_set_skip_taskbar_hint")
			gdkX11SurfaceSetSkipTaskbarHintSet = true
		}
		provider := gtkCssProviderNew()
		gtkCssProviderLoadFromString(provider, cstr(lookCSS))
		// GTK_STYLE_PROVIDER_PRIORITY_APPLICATION.
		gtkStyleContextAddForDisplay(gdkDisplayGetDefault(), provider, 600)
	})
	// noTaskListHint is logged once where the session has no hint to keep a window off its task list.
	noTaskListHint sync.Once
)

// applyLook opens the window as opts ask: transparent, without a shadow, off the task list.
func (w *window) applyLook(opts Options) {
	if opts.Transparent {
		w.setTransparent(true)
	}
	if opts.NoShadow {
		w.setShadow(false)
	}
	if opts.SkipTaskbar {
		w.setSkipTaskbar(true)
	}
}

// setCSSClass gives the window class, or takes it away.
func (w *window) setCSSClass(class string, on bool) {
	bindLook()
	if on {
		gtkWidgetAddCSSClass(w.win, cstr(class))
	} else {
		gtkWidgetRemoveCSSClass(w.win, cstr(class))
	}
}

// setTransparent makes the window undecorated and clear wherever the app draws nothing, or gives it back its theme's background and, with the default title bar, its decorations.
func (w *window) setTransparent(on bool) {
	if w.win == 0 {
		return
	}
	w.transparent = on
	w.setCSSClass("background", !on)
	w.setCSSClass("nsgo-transparent", on)
	gtkWindowSetDecorated(w.win, boolInt(!on && w.chrome.titleBar == driver.TitleBarDefault))
	w.showMenuBar()
	w.updateEnvironment()
}

// setShadow keeps the shadow GTK draws around the window, or clears it.
func (w *window) setShadow(on bool) {
	if w.win != 0 {
		w.setCSSClass("nsgo-no-shadow", !on)
	}
}

// setSkipTaskbar keeps the window off the X11 task list, or puts it back; where GDK does not draw on X11 it logs once that it cannot.
func (w *window) setSkipTaskbar(on bool) {
	if w.win == 0 {
		return
	}
	bindLook()
	if _, _, ok := w.x11Window(); !ok || !gdkX11SurfaceSetSkipTaskbarHintSet {
		if on {
			noTaskListHint.Do(func() {
				log.Print("nsgo: only X11 has a hint to keep a window off the task list: the window stays in it")
			})
		}
		return
	}
	if surface := gtkNativeGetSurface(w.win); surface != 0 {
		gdkX11SurfaceSetSkipTaskbarHint(surface, boolInt(on))
	}
}
