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
	postForeign(t, true)
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
	postForeign(t, false)
	app.Settle()
	app.Find("text:0")
	app.Key("x")
	app.Find("text:1")
}

// postForeign posts an AppKit event without going through the test host,
// as an event from the user's hardware arrives. It targets only this
// test process's window and never posts a system-wide event.
func postForeign(t *testing.T, mouse bool) {
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
		number := objc.Send[int](window, sel("windowNumber"))
		events := objc.ID(apple.Class("NSEvent"))
		now := func() float64 {
			return objc.Send[float64](objc.ID(apple.Class("NSProcessInfo")).Send(sel("processInfo")), sel("systemUptime"))
		}
		if mouse {
			at := objc.Send[apple.CGPoint](window.Send(sel("contentView")), sel("convertPoint:toView:"),
				apple.CGPoint{X: 40, Y: 40}, objc.ID(0))
			for _, kind := range []uint{5, 1, 2} {
				event := events.Send(sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
					kind, at, uint(0), now(), number, objc.ID(0), 0, 1, float32(1))
				app.Send(sel("postEvent:atStart:"), event, false)
			}
		} else {
			for _, kind := range []uint{10, 11} {
				event := events.Send(sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
					kind, apple.CGPoint{}, uint(0), now(), number, objc.ID(0), apple.NSString("x"), apple.NSString("x"), uint64(7)<<16)
				app.Send(sel("postEvent:atStart:"), event, false)
			}
		}
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
