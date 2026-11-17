//go:build linux && !android

package linux

import (
	"testing"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
)

// TestTransparentWindowLook opens a window transparent and off the task list: it is undecorated without the theme's background, and its environment says transparency shows where the display composites with alpha and the task list hint exists where GDK draws on X11. Made framed again it has both back.
func TestTransparentWindowLook(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	var (
		hasCSSClass  func(widget uintptr, class *byte) int32
		getDecorated func(window uintptr) int32
	)
	purego.RegisterLibFunc(&hasCSSClass, gtkLib, "gtk_widget_has_css_class")
	purego.RegisterLibFunc(&getDecorated, gtkLib, "gtk_window_get_decorated")
	app := func(cx *core.Scope) core.Node { return core.View{} }
	w := startTestWindow(t, app, Options{Title: "nsgo-transparent", Width: 160, Height: 100, Transparent: true, SkipTaskbar: true})
	w.onLoop(t, func() {
		if hasCSSClass(w.win, cstr("nsgo-transparent")) == 0 || hasCSSClass(w.win, cstr("background")) != 0 || getDecorated(w.win) != 0 {
			t.Error("a transparent window keeps its decorations or its theme's background")
		}
		env := w.environment()
		if env.CanMakeWindowsTransparent != displayComposites() || env.CanSkipTaskbar != skipTaskbarHint() {
			t.Errorf("the environment reports transparency %v and the task list %v; the display composites %v and has the hint %v",
				env.CanMakeWindowsTransparent, env.CanSkipTaskbar, displayComposites(), skipTaskbarHint())
		}
		w.setTransparent(false)
		if hasCSSClass(w.win, cstr("nsgo-transparent")) != 0 || hasCSSClass(w.win, cstr("background")) == 0 || getDecorated(w.win) == 0 {
			t.Error("a window made framed again lacks its decorations or its theme's background")
		}
	})
}
