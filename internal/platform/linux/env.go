//go:build linux && !android

package linux

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/speechd"
)

// speakAvailable is Environment.Speak, read once: speech-dispatcher listens or is installed.
var speakAvailable = sync.OnceValue(speechd.Available)

const (
	gtkTextDirLTR = 1
	gtkTextDirRTL = 2
	// gtk-xft-dpi is in 1024ths of a point per inch.
	xftDPIUnit = 1024
)

// environment reads what GTK and the session say about where the app
// runs. There is no software keyboard, so its inset stays 0.
func (w *window) environment() driver.Environment {
	dark, reduce, contrast, dpi := false, false, false, int32(96*xftDPIUnit)
	theme := ""
	if settings := gtkSettingsGetDefault(); settings != 0 {
		if colorScheme() {
			dark = settingInt(settings, "gtk-interface-color-scheme") == schemeDark
		} else {
			dark = settingBool(settings, "gtk-application-prefer-dark-theme")
		}
		reduce = !settingBool(settings, "gtk-enable-animations")
		dpi = settingInt(settings, "gtk-xft-dpi")
		theme = settingString(settings, "gtk-theme-name")
		if colorScheme() {
			contrast = settingInt(settings, "gtk-interface-contrast") == contrastMore
		}
	}
	if strings.Contains(strings.ToLower(theme), "dark") || strings.Contains(strings.ToLower(os.Getenv("GTK_THEME")), "dark") {
		dark = true
	}
	// GTK 4.20 and later have the session's contrast as a setting. Before it, GNOME's High Contrast is read where its schema is installed, and other desktops say it with a high-contrast theme.
	contrast = contrast || gnomeHighContrast()
	for _, name := range []string{theme, os.Getenv("GTK_THEME")} {
		name = strings.ToLower(name)
		if strings.Contains(name, "highcontrast") || strings.Contains(name, "high-contrast") {
			contrast = true
		}
	}
	scale := 1.0
	if surface := gtkNativeGetSurface(gtkWidgetGetNative(w.surface)); surface != 0 {
		scale = gdkSurfaceGetScale(surface)
	} else if f := gtkWidgetGetScaleFactor(w.surface); f > 0 {
		scale = float64(f)
	}
	if scale <= 0 {
		scale = 1
	}
	fontScale := float32(1)
	if dpi > 0 {
		fontScale = float32(float64(dpi) / float64(96*xftDPIUnit))
	}
	locale := sessionLocale()
	var frame driver.WindowFrame
	if w.win != 0 {
		var width, height int32
		gtkWindowGetDefaultSize(w.win, &width, &height)
		frame.Width, frame.Height = driver.Points(width), driver.Points(height)
	}
	minimized := false
	if w.surface != 0 {
		if top := gtkNativeGetSurface(gtkWidgetGetNative(w.surface)); top != 0 {
			minimized = gdkToplevelGetState(top)&toplevelMinimized != 0
		}
	}
	front, followed := frontmostApp()
	return driver.Environment{
		Dark:             dark,
		FontScale:        fontScale,
		ReduceMotion:     reduce,
		IncreaseContrast: contrast,
		RTL:              systemDirection() == gtkTextDirRTL || rightToLeft(locale),
		PixelRatio:       float32(scale),
		Locale:           locale,
		LocaleSettings:   localeSettings(),
		Displays:         displays(),
		OS:               driver.OSLinux,
		HasWebView:       true,
		OSVersion:        osVersion(),
		Idiom:            driver.IdiomDesktop,
		TitleBar:         w.titleBarArea(),
		Maximized:        w.win != 0 && gtkWindowIsMaximized(w.win) != 0,
		FullScreen:       w.chrome.fullScreen,
		Minimized:        minimized,
		WindowFrame:      frame,
		Lifecycle:        w.lifecycle(),
		Speak:            speakAvailable(),

		// GTK keeps only a window's size (setFrame), so no window is placed, and a Wayland compositor stacks its windows itself.
		CanSetWindowLevel:      w.canSetLevel(),
		CanSetWindowWorkspaces: w.canSetWorkspaces(),

		CanMakeWindowsTransparent: w.ownWindow() && displayComposites(),
		CanSkipTaskbar:            w.ownWindow() && skipTaskbarHint(),
		FrontmostApp:              front,

		// Only an X11 session says which window is in front.
		CanReadFrontmostApp: followed,
	}
}

// displays reads GDK's monitors in the same logical coordinate space as
// WindowFrame. GTK exposes no primary-monitor flag, so the first monitor is
// the primary one, matching GTK's ordering.
func displays() []driver.DisplayInfo {
	display := gdkDisplayGetDefault()
	if display == 0 {
		return nil
	}
	model := gdkDisplayGetMonitors(display)
	if model == 0 {
		return nil
	}
	watchDisplays(model)
	count := gListModelGetNItems(model)
	out := make([]driver.DisplayInfo, 0, count)
	for i := uint32(0); i < count; i++ {
		monitor := gListModelGetItem(model, i)
		if monitor == 0 {
			continue
		}
		watchMonitor(monitor)
		var frame gdkMonitorRectangle
		gdkMonitorGetGeometry(monitor, &frame)
		// Wayland reports no work area, so there the whole monitor is the visible one.
		visible := frame
		if gdkX11MonitorGetWorkarea != nil && gTypeIsA(monitor, gdkX11MonitorGetType()) != 0 {
			gdkX11MonitorGetWorkarea(monitor, &visible)
		}
		// The fractional scale, as PixelRatio's: the scale factor is its ceiling.
		var scale float32
		if gdkMonitorGetScale != nil {
			scale = float32(gdkMonitorGetScale(monitor))
		} else {
			scale = float32(gdkMonitorGetScaleFactor(monitor))
		}
		if scale <= 0 {
			scale = 1
		}
		point := func(r gdkMonitorRectangle) driver.Bounds {
			return driver.Bounds{Origin: driver.Point{X: driver.Points(float32(r.X)), Y: driver.Points(float32(r.Y))}, Size: driver.Size{Width: driver.Points(float32(r.Width)), Height: driver.Points(float32(r.Height))}}
		}
		// The connector, as DP-1 or HDMI-A-1, names the display across runs; the object's address only while it is connected.
		id := ""
		if c := gdkMonitorGetConnector(monitor); c != 0 {
			id = goString(c)
		}
		if id == "" {
			id = fmt.Sprintf("%x", monitor)
		}
		out = append(out, driver.DisplayInfo{ID: id, Frame: point(frame), VisibleFrame: point(visible), Scale: scale, Main: i == 0})
		gObjectUnref(monitor)
	}
	return out
}

var watchedDisplays uintptr

func watchDisplays(model uintptr) {
	if watchedDisplays == model {
		return
	}
	watchedDisplays = model
	gSignalConnect(model, "items-changed", cbDisplaysChanged, 0)
}

// watchMonitor rereads the displays when monitor moves, resizes or changes its scale, which GDK reports on the monitor rather than as a change to the model's items. The mark it leaves on the monitor goes with it.
func watchMonitor(monitor uintptr) {
	key := cstr("nsgo-displays-watched")
	if gObjectGetData(monitor, key) != 0 {
		return
	}
	gObjectSetData(monitor, key, 1)
	for _, signal := range []string{"notify::geometry", "notify::scale", "notify::scale-factor", "invalidate"} {
		gSignalConnect(monitor, signal, cbDisplaysChanged, 0)
	}
}

func displaysChangedCallback(_, _, _, _ uintptr) {
	windowHandles.Range(func(_, value any) bool {
		if w := value.(*window); !w.closing {
			w.updateEnvironment()
		}
		return true
	})
}

// GdkToplevelState's flags that hide a window: minimized, and suspended
// (on another workspace or covered, where the compositor says).
const toplevelMinimized, toplevelSuspended = 1 << 0, 1 << 16

// lifecycle is whether the app is in use: in the background while the
// surface is unmapped or its window minimized or suspended, active while
// one of the app's windows has the focus.
func (w *window) lifecycle() driver.Lifecycle {
	if w.primary && w.opts.Hidden {
		return appLifecycle()
	}
	if w.surface == 0 || gtkWidgetGetMapped(w.surface) == 0 {
		return driver.LifecycleBackground
	}
	if top := gtkNativeGetSurface(gtkWidgetGetNative(w.surface)); top != 0 &&
		gdkToplevelGetState(top)&(toplevelMinimized|toplevelSuspended) != 0 {
		return driver.LifecycleBackground
	}
	active := false
	windowHandles.Range(func(_, v any) bool {
		if other := v.(*window); !other.closing && other.rootWindow() != 0 && gtkWindowIsActive(other.rootWindow()) != 0 {
			active = true
		}
		return !active
	})
	if active {
		return driver.LifecycleActive
	}
	return driver.LifecycleInactive
}

// appLifecycle is the lifecycle of the app's own surface, which shows in no window (Options.Hidden): its windows', active while one has the focus and in the background while none shows.
func appLifecycle() driver.Lifecycle {
	l := driver.LifecycleBackground
	windowHandles.Range(func(_, v any) bool {
		w := v.(*window)
		if w.closing || w.primary && w.opts.Hidden {
			return true
		}
		switch w.lifecycle() {
		case driver.LifecycleActive:
			l = driver.LifecycleActive
			return false
		case driver.LifecycleInactive:
			l = driver.LifecycleInactive
		}
		return true
	})
	return l
}

// updateAppSurface reads the environment of the app's own surface again as one of its windows shows, hides or goes.
func updateAppSurface() {
	windowHandles.Range(func(_, v any) bool {
		if w := v.(*window); w.primary && w.opts.Hidden && !w.closing {
			w.updateEnvironment()
		}
		return true
	})
}

// rootWindow is the GtkWindow the surface shows in, the host's for an
// embedded one; 0 while it is in none.
func (w *window) rootWindow() uintptr {
	if w.win != 0 {
		return w.win
	}
	if w.surface == 0 {
		return 0
	}
	return gtkWidgetGetRoot(w.surface)
}

// mappedCallback follows the surface's map and unmap, and on the first
// map its window's focus and its toplevel's state, which exist only once
// it is in a window.
func mappedCallback(widget, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing {
		return
	}
	if gtkWidgetGetMapped(w.surface) != 0 && w.watchedRoot == 0 {
		if root := w.rootWindow(); root != 0 {
			w.watchedRoot = root
			// The host's window may outlive this surface. GTK disconnects these broadcasts when either endpoint goes.
			gSignalConnectObj(root, cstr("notify::is-active"), cbActivity, w.surface, 0)
			if top := gtkNativeGetSurface(gtkWidgetGetNative(w.surface)); top != 0 {
				gSignalConnectObj(top, cstr("notify::state"), cbActivity, w.surface, 0)
			}
		}
	}
	w.updateEnvironment()
	if !w.primary {
		updateAppSurface()
	}
}

// activityCallback reads every window's environment again as one's focus
// or toplevel state changes, the app being active while any is.
func activityCallback(object, pspec, data uintptr) {
	windowHandles.Range(func(_, v any) bool {
		if w := v.(*window); !w.closing {
			active := w.win != 0 && gtkWindowIsActive(w.win) != 0
			if active {
				w.focused = true
			}
			if w.kind == driver.SurfacePanel && w.hideOnBlur && w.visible && w.focused && !active {
				w.visible = false
				gtkWidgetSetVisible(w.win, 0)
				if w.engine != nil {
					w.engine.Dispatch(driver.Event{Kind: driver.EventWindowHidden})
				}
			}
			w.updateEnvironment()
		}
		return true
	})
}

// updateEnvironment reads the environment again and reports it when it
// changed.
func (w *window) updateEnvironment() {
	env := w.environment()
	if env.Equal(w.env) {
		return
	}
	w.env = env
	if w.engine != nil {
		w.engine.SetEnvironment(env)
	}
}

// settingBool reads a gboolean property of settings.
func settingBool(settings uintptr, name string) bool {
	var v int32
	gObjectGet(settings, uintptr(unsafe.Pointer(cstr(name))), uintptr(unsafe.Pointer(&v)), 0)
	return v != 0
}

// settingInt reads an int property of settings.
func settingInt(settings uintptr, name string) int32 {
	var v int32
	gObjectGet(settings, uintptr(unsafe.Pointer(cstr(name))), uintptr(unsafe.Pointer(&v)), 0)
	return v
}

// settingString reads a string property of settings; the copy GTK makes
// is freed.
func settingString(settings uintptr, name string) string {
	var p uintptr
	gObjectGet(settings, uintptr(unsafe.Pointer(cstr(name))), uintptr(unsafe.Pointer(&p)), 0)
	s := goString(p)
	if p != 0 {
		gFree(p)
	}
	return s
}

// settingsChangedCallback rereads the environment and the UI font after a GTK setting changed.
func settingsChangedCallback(settings, pspec, data uintptr) {
	if readUIFont() {
		uiFontChanged()
	}
	if w := windowOf(data); w != nil && !w.closing {
		w.updateEnvironment()
	}
}

// uiFontChanged has every app showing prepare its text again in the UI font GTK's settings changed to, and every canvas draw its text in it.
func uiFontChanged() {
	windowHandles.Range(func(_, v any) bool {
		w := v.(*window)
		if w.closing {
			return true
		}
		if w.engine != nil {
			w.engine.FontsChanged()
		}
		for _, cv := range w.views {
			if cv != nil && cv.drawing != nil && cv.widget != 0 {
				gtkWidgetQueueDraw(cv.widget)
			}
		}
		return true
	})
}

// windowStateCallback rereads the environment after the window is
// maximized, restored or made full screen, by the app or the window
// manager.
func windowStateCallback(window, pspec, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing {
		return
	}
	w.chrome.fullScreen = gtkWindowIsFullscreen(w.win) != 0
	w.updateEnvironment()
}

// sessionLocale returns the user's locale as a BCP 47 tag, such as
// "en-US".
func sessionLocale() string {
	locale := ""
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			locale = v
			break
		}
	}
	// Drop the encoding and modifier: "de_DE.UTF-8@euro" is "de-DE".
	if i := strings.IndexAny(locale, ".@"); i >= 0 {
		locale = locale[:i]
	}
	return strings.ReplaceAll(locale, "_", "-")
}

// localeSettings is what the session sets apart from the language: the
// region of LC_TIME's formats, LC_MEASUREMENT's system, and GNOME's
// clock-format over the hours.
func localeSettings() driver.LocaleSettings {
	region := func(category string) string {
		for _, key := range []string{"LC_ALL", category, "LANG"} {
			if v := os.Getenv(key); v != "" {
				v, _, _ = strings.Cut(v, ".")
				v, _, _ = strings.Cut(v, "@")
				if _, r, ok := strings.Cut(v, "_"); ok && len(r) == 2 {
					return strings.ToUpper(r)
				}
				return ""
			}
		}
		return ""
	}
	s := driver.LocaleSettings{Region: region("LC_TIME")}
	switch region("LC_MEASUREMENT") {
	case "":
	case "US", "LR", "MM":
		s.MeasurementSystem = "us"
	case "GB":
		s.MeasurementSystem = "uk"
	default:
		s.MeasurementSystem = "metric"
	}
	switch gnomeClockFormat() {
	case "24h":
		s.HourCycle = "h23"
	case "12h":
		s.HourCycle = "h12"
	}
	return s
}

// gnomeClockFormat is GNOME's clock-format setting, "" where its schema
// is not installed.
func gnomeClockFormat() string {
	const schema = "org.gnome.desktop.interface"
	gnomeSettingsOnce.Do(func() {
		var lookup, source, settingsNew uintptr
		var err error
		if source, err = purego.Dlsym(gioLib, "g_settings_schema_source_get_default"); err != nil {
			return
		}
		if lookup, err = purego.Dlsym(gioLib, "g_settings_schema_source_lookup"); err != nil {
			return
		}
		if settingsNew, err = purego.Dlsym(gioLib, "g_settings_new"); err != nil {
			return
		}
		src, _, _ := purego.SyscallN(source)
		if src == 0 {
			return
		}
		if found, _, _ := purego.SyscallN(lookup, src, uintptr(unsafe.Pointer(cstr(schema))), 1); found == 0 {
			return
		}
		gnomeSettings, _, _ = purego.SyscallN(settingsNew, uintptr(unsafe.Pointer(cstr(schema))))
		gnomeGetString, _ = purego.Dlsym(gioLib, "g_settings_get_string")
	})
	if gnomeSettings == 0 || gnomeGetString == 0 {
		return ""
	}
	p, _, _ := purego.SyscallN(gnomeGetString, gnomeSettings, uintptr(unsafe.Pointer(cstr("clock-format"))))
	s := goString(p)
	if p != 0 {
		gFree(p)
	}
	return s
}

var (
	gnomeSettingsOnce             sync.Once
	gnomeSettings, gnomeGetString uintptr
)

var (
	gnomeA11yOnce                      sync.Once
	gnomeA11ySettings, gnomeGetBoolean uintptr
)

// gnomeHighContrast reads GNOME's optional accessibility setting without linking to GSettings at build time. Other GTK desktops use their theme name, handled by environment.
func gnomeHighContrast() bool {
	gnomeA11yOnce.Do(func() {
		var lookup, source, settingsNew uintptr
		var err error
		if source, err = purego.Dlsym(gioLib, "g_settings_schema_source_get_default"); err != nil {
			return
		}
		if lookup, err = purego.Dlsym(gioLib, "g_settings_schema_source_lookup"); err != nil {
			return
		}
		if settingsNew, err = purego.Dlsym(gioLib, "g_settings_new"); err != nil {
			return
		}
		src, _, _ := purego.SyscallN(source)
		if src == 0 {
			return
		}
		if found, _, _ := purego.SyscallN(lookup, src, uintptr(unsafe.Pointer(cstr("org.gnome.desktop.a11y.interface"))), 1); found == 0 {
			return
		}
		gnomeA11ySettings, _, _ = purego.SyscallN(settingsNew, uintptr(unsafe.Pointer(cstr("org.gnome.desktop.a11y.interface"))))
		gnomeGetBoolean, _ = purego.Dlsym(gioLib, "g_settings_get_boolean")
	})
	if gnomeA11ySettings == 0 || gnomeGetBoolean == 0 {
		return false
	}
	v, _, _ := purego.SyscallN(gnomeGetBoolean, gnomeA11ySettings, uintptr(unsafe.Pointer(cstr("high-contrast"))))
	return v != 0
}

func gnomeA11ySettingsForWatch() uintptr {
	gnomeHighContrast()
	return gnomeA11ySettings
}

// rightToLeft reports whether locale's language reads right to left.
func rightToLeft(locale string) bool {
	language, _, _ := strings.Cut(strings.ToLower(locale), "-")
	switch language {
	case "ar", "arc", "ckb", "dv", "fa", "he", "ks", "ku", "ps", "sd", "ug", "ur", "yi":
		return true
	}
	return false
}
