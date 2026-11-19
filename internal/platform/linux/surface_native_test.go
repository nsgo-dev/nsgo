//go:build linux && !android

package linux

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestOpenedWindowCloseIsTheApps asks to close an opened window from its menu and title bar: the window stays while OnClosing cancels, and goes with its handle once the app lets it close.
func TestOpenedWindowCloseIsTheApps(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	var asked atomic.Int32
	var allow atomic.Bool
	app := core.AppRoot(func(cx *core.Scope) {
		cx.SetShutdownMode(core.ShutdownOnExplicit)
		win := cx.OpenWindow(core.WindowOptions{Title: "nsgo-opened", Width: 160, Height: 120}, func(cx *core.Scope) core.Node {
			cx.SetApplicationMenu([]core.MenuItem{{Label: "File", Items: []core.MenuItem{{Role: core.MenuClose}}}})
			return core.View{Style: core.Style{Width: driver.Pt(160), Height: driver.Pt(120)}}
		})
		win.OnClosing(cx, func(c *core.WindowClosing) {
			asked.Add(1)
			if !allow.Load() {
				c.Cancel()
			}
		})
	})
	w := startTestWindow(t, app, Options{Title: "nsgo-app", Width: 120, Height: 80, Hidden: true})
	defer w.onLoop(t, w.close)
	var opened *window
	for deadline := time.Now().Add(10 * time.Second); opened == nil; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the window did not open")
		}
		w.onLoop(t, func() {
			windowHandles.Range(func(_, v any) bool {
				if o := v.(*window); !o.primary && !o.embedded && o.win != 0 && o.kind == driver.SurfaceWindow {
					opened = o
				}
				return opened == nil
			})
		})
	}
	wait := func(what string, done func() bool) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			var ok bool
			w.onLoop(t, func() { ok = done() })
			if ok {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal(what)
			}
		}
	}
	wait("the window did not show", func() bool { return gtkWidgetGetVisible(opened.win) != 0 })
	w.onLoop(t, func() {
		if w.lifecycle() == driver.LifecycleBackground {
			t.Error("the app's surface is in the background while its window shows")
		}
	})
	// The Close role's own shortcut, as Ctrl+W reaches it.
	wait("the window's menu has no Close shortcut", func() bool { _, ok := opened.chrome.keys["mod+w"]; return ok })
	w.onLoop(t, func() {
		if !opened.menuShortcut("mod+w") {
			t.Error("mod+w did not reach the menu's Close")
		}
	})
	for deadline := time.Now().Add(10 * time.Second); asked.Load() == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("menu Close did not run OnClosing")
		}
	}
	w.onLoop(t, func() {
		if opened.win == 0 || gtkWidgetGetVisible(opened.win) == 0 || windowOf(opened.handle) == nil {
			t.Error("a canceled menu Close took the window away")
		}
	})
	w.onLoop(t, func() { gtkWindowClose(opened.win) })
	for deadline := time.Now().Add(10 * time.Second); asked.Load() < 2; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("OnClosing did not run")
		}
	}
	// A frame for anything the close would have done.
	time.Sleep(100 * time.Millisecond)
	w.onLoop(t, func() {
		if opened.win == 0 || gtkWidgetGetVisible(opened.win) == 0 || windowOf(opened.handle) == nil {
			t.Error("a canceled close took the window away")
		}
	})
	allow.Store(true)
	w.onLoop(t, func() { gtkWindowClose(opened.win) })
	wait("the window stayed after its close", func() bool { return opened.win == 0 && windowOf(opened.handle) == nil })
	if asked.Load() != 3 {
		t.Errorf("OnClosing ran %d times", asked.Load())
	}
}
