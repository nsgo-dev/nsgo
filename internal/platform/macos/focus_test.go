//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// WebKit can make its content view the first responder. Observe the host
// window, including one belonging to another app, rather than subclassing it.
func TestWebFocusFollowsTheHostWindow(t *testing.T) {
	onMain(func() {
		loadAppKit()
		objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication"))
		observeApp()
		window := func() objc.ID {
			return objc.ID(apple.Class("NSWindow")).Send(selAlloc).Send(sel("initWithContentRect:styleMask:backing:defer:"),
				apple.Rect(0, 0, 320, 240), uint(0), uint(2), false)
		}
		first, second := window(), window()
		defer first.Send(selRelease)
		defer second.Send(selRelease)
		holder := objc.ID(apple.Class("NSView")).Send(selNew)
		defer holder.Send(selRelease)
		first.Send(sel("setContentView:"), holder)
		s := &surface{window: first, native: holder, embedded: true}
		views[holder] = &view{surface: s}
		defer delete(views, holder)
		web := apple.NewWeb(holder)
		defer web.Close()
		web.Dispatch = func(driver.Event) {}
		web.Configure(&driver.WebConfig{})
		v := &view{surface: s, kind: driver.KindWeb, native: holder}
		views[web.View()] = v
		defer delete(views, web.View())
		s.observeWebFocus()
		defer func() { s.stopWebFocus() }()
		focus := func(w, responder objc.ID, want *view) {
			t.Helper()
			if !objc.Send[bool](w, sel("makeFirstResponder:"), responder) {
				t.Fatal("the window refused the focus")
			}
			if s.webFocus != want {
				t.Errorf("focused page = %p, want %p", s.webFocus, want)
			}
		}
		focus(first, web.View(), v)
		focus(first, first, nil)
		focus(first, web.View(), v)
		// Reattaching an embedded surface must stop watching the old window.
		second.Send(sel("setContentView:"), holder)
		s.window = second
		s.observeWebFocus()
		focus(second, web.View(), v)
		focus(first, first, v)
		focus(second, second, nil)
	})
}
