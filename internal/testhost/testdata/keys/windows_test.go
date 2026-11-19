package keys_test

import (
	"sync/atomic"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// desktopApp returns the options a test of the windows an app opens itself starts it with: headless a macOS environment, which opens them, and on the macOS target none, as the app opens them there as it runs. The other targets' test hosts show the app in one window, so it skips the test there.
func desktopApp(t *testing.T) []nsgotest.Option {
	t.Helper()
	switch target := nsgotest.Target(); target {
	case "":
		return []nsgotest.Option{nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop})}
	case "macos":
		return nil
	default:
		t.Skipf("the %s test host opens no windows of the app's", target)
		return nil
	}
}

// A key the main window takes reaches an AllSurfaces binding of another window the app opened, without a test's Environment on the macOS target, and steps act on the main window, which opens at the desktop's default size there (#763).
func TestAllSurfacesBindingOfAnOpenedWindowHearsTheMainWindowsKey(t *testing.T) {
	opts := desktopApp(t)
	var calls atomic.Int32
	var size atomic.Value
	start := func(cx *core.Scope) {
		cx.SetMainView(func(cx *core.Scope) core.Node {
			size.Store(cx.Env().Window())
			return core.Text{Content: "main", TestID: "main"}
		})
		desktop.OpenWindow(cx, desktop.WindowOptions{Title: "Other", Width: 200, Height: 100}, func(cx *core.Scope) core.Node {
			cx.OnKeyWith("mod+shift+y", core.KeyOptions{AllSurfaces: true}, func() { calls.Add(1) })
			return core.Text{Content: "other", TestID: "other"}
		})
	}
	app := nsgotest.StartApp(t, start, opts...)
	app.Find("id:main")
	if len(app.Query("id:other")) != 0 {
		t.Fatal("steps reach another window than the main one")
	}
	app.Key("mod+shift+y")
	if got := calls.Load(); got != 1 {
		t.Fatalf("the AllSurfaces binding ran %d times, want 1", got)
	}
	if want := (core.Size{Width: 480, Height: 640}); nsgotest.Target() == "macos" && size.Load() != want {
		t.Errorf("the main window is %v, want the desktop's default %v", size.Load(), want)
	}
}

// Closing the main window asks its OnClosing handlers, which may keep it open, and an accepted close leaves no view to act on, the app quitting as its last window closes, rather than ending the test's process (#771). A root view shows in the main window too, as Run shows it.
func TestMainWindowCloseAsksOnClosing(t *testing.T) {
	opts := desktopApp(t)
	var asked atomic.Int32
	var keep atomic.Bool
	keep.Store(true)
	view := func(cx *core.Scope) core.Node {
		w := desktop.CurrentWindow(cx)
		if w == nil {
			return core.Text{Content: "no window", TestID: "main"}
		}
		w.OnClosing(cx, func(c *desktop.WindowClosing) {
			asked.Add(1)
			if keep.Load() {
				c.Cancel()
			}
		})
		desktop.SetApplicationMenu(cx, []desktop.MenuItem{{Label: "File", Items: []desktop.MenuItem{{Role: desktop.MenuClose}}}})
		return core.Text{Content: "main", TestID: "main"}
	}
	app := nsgotest.Start(t, view, opts...)
	if got := app.Find("id:main").Text; got != "main" {
		t.Fatalf("the main view shows %q: it is in no window", got)
	}
	app.Key("mod+w")
	if got := asked.Load(); got != 1 {
		t.Fatalf("closing the main window asked %d handlers, want 1", got)
	}
	app.Find("id:main")
	keep.Store(false)
	app.Key("mod+w")
	if got := asked.Load(); got != 2 {
		t.Fatalf("closing the main window again asked %d handlers, want 2", got)
	}
	if len(app.Query("id:main")) != 0 {
		t.Fatal("steps still reach the closed window's views")
	}
	if app.Quits != 1 {
		t.Fatalf("the app quit %d times as its last window closed, want 1", app.Quits)
	}
}
