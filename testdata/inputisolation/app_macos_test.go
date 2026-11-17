//go:build darwin && !ios

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestForeignClicksDoNotChangeTheScene(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("checks AppKit's native input queue")
	}
	app := nsgotest.Start(t, Root)
	postForeign(t, 5, 1, 2)
	app.Settle()
	app.Find("text:0")
	app.Press("id:add")
	app.Find("text:1")
}

func TestForeignKeysDoNotChangeTheScene(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("checks AppKit's native input queue")
	}
	app := nsgotest.Start(t, Root)
	postForeign(t, 10, 11)
	app.Settle()
	app.Find("text:0")
	app.Key("x")
	app.Find("text:1")
}

func TestScriptedKeysReachANonKeyWindow(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("checks AppKit's background key delivery")
	}
	app := nsgotest.Start(t, Root)
	postForeign(t, 1) // The user holds a mouse button while the test types.
	app.Settle()
	var key bool
	withTestWindow(t, func(window objc.ID) {
		window.Send(apple.Sel("makeFirstResponder:"), objc.ID(0))
		window.Send(apple.Sel("resignKeyWindow"))
		key = objc.Send[bool](window, apple.Sel("isKeyWindow"))
	})
	if key {
		t.Fatal("the test window did not resign its key status")
	}
	app.Key("tab")
	if !app.Find("id:add").Focused {
		t.Fatal("the scripted Tab did not reach the background window")
	}
	app.Key("space")
	app.Find("text:1")
	app.Key("x")
	app.Find("text:2")
	withTestWindow(t, func(window objc.ID) {
		window.Send(apple.Sel("makeFirstResponder:"), objc.ID(0))
	})
	app.Key("shift+tab")
	if !app.Find("id:add").Focused {
		t.Fatal("the scripted Shift+Tab did not reach the background window")
	}
	app.Key("space")
	app.Find("text:3")
	app.Key("mod+k")
	app.Find("text:4")
}

// postForeign posts an AppKit event without going through the test host, as an event from the user's hardware arrives. It targets only this test process's window and never posts a system-wide event.
func postForeign(t *testing.T, kinds ...uint) {
	t.Helper()
	withTestWindow(t, func(window objc.ID) {
		sel := apple.Sel
		app := objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication"))
		number := objc.Send[int](window, sel("windowNumber"))
		events := objc.ID(apple.Class("NSEvent"))
		now := func() float64 {
			return objc.Send[float64](objc.ID(apple.Class("NSProcessInfo")).Send(sel("processInfo")), sel("systemUptime"))
		}
		at := objc.Send[apple.CGPoint](window.Send(sel("contentView")), sel("convertPoint:toView:"),
			apple.CGPoint{X: 40, Y: 40}, objc.ID(0))
		for _, kind := range kinds {
			var event objc.ID
			if kind == 10 || kind == 11 {
				event = apple.KeyEvent(kind, 0, now(), number, "x", "x", false, 7)
			} else {
				event = events.Send(sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
					kind, at, uint(0), now(), number, objc.ID(0), 0, 1, float32(1))
			}
			app.Send(sel("postEvent:atStart:"), event, false)
		}
	})
}

func withTestWindow(t *testing.T, fn func(objc.ID)) {
	t.Helper()
	done := make(chan error, 1)
	apple.OnMain(func() {
		sel := apple.Sel
		app := objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication"))
		windows := app.Send(sel("windows"))
		var window objc.ID
		for i := range objc.Send[int](windows, sel("count")) {
			candidate := windows.Send(sel("objectAtIndex:"), i)
			if apple.GoString(candidate.Send(sel("title"))) == "NSGo test" {
				window = candidate
				break
			}
		}
		if window == 0 {
			done <- fmt.Errorf("the test has no window")
			return
		}
		fn(window)
		done <- nil
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AppKit did not post the foreign input")
	}
}
