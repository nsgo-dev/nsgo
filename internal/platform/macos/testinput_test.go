//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func testMouse(kind uint, at float64, window int) objc.ID {
	return objc.ID(apple.Class("NSEvent")).Send(
		sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
		kind, apple.CGPoint{}, uint(0), at, window, objc.ID(0), 0, 1, float32(1))
}

func TestScriptedInputSurvivesAppKitCopies(t *testing.T) {
	onMain(func() {
		loadAppKit()
		var history scriptedInputs
		event := testMouse(evLeftDown, 12, 0)
		history.remember(event)
		copy := event.Send(sel("copy"))
		defer copy.Send(selRelease)
		if !history.contains(copy) {
			t.Error("AppKit's copy lost its scripted identity")
		}
		for _, c := range []struct {
			name   string
			kind   uint
			at     float64
			window int
			want   bool
		}{
			{"nanosecond rounding", evLeftDown, 12 + 1e-9, 0, true},
			{"foreign timestamp", evLeftDown, 13, 0, false},
			{"another input type", evLeftUp, 12, 0, false},
			{"another window", evLeftDown, 12, 1, false},
		} {
			if got := history.contains(testMouse(c.kind, c.at, c.window)); got != c.want {
				t.Errorf("%s: recognized %v, want %v", c.name, got, c.want)
			}
		}
		for i := range len(history.events) {
			history.remember(testMouse(evLeftDown, float64(20+i), 0))
		}
		if history.contains(copy) || !history.contains(testMouse(evLeftDown, 83, 0)) {
			t.Error("the bounded history did not evict only the old input")
		}
	})
}

func TestInputIsolationKeepsApplicationEvents(t *testing.T) {
	onMain(func() {
		loadAppKit()
		before := testInputFilter
		defer func() { testInputFilter = before }()
		event := testMouse(evLeftDown, 12, 0)
		testInputFilter = nil
		if !inputAllowed(event) {
			t.Error("ordinary applications rejected their input")
		}
		var history scriptedInputs
		testInputFilter = history.contains
		if inputAllowed(event) || (&surface{}).filterEvent(event) != 0 {
			t.Error("a test accepted foreign input before its window monitor")
		}
		history.remember(event)
		if !inputAllowed(event) || !inputAllowed(0) {
			t.Error("a test rejected its own input")
		}
		application := objc.ID(apple.Class("NSEvent")).Send(
			sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"),
			uint(15), apple.CGPoint{}, uint(0), float64(12), 0, objc.ID(0), int16(0), 0, 0)
		if application == 0 || !inputAllowed(application) {
			t.Error("the test suppressed AppKit's own application event")
		}
	})
}

func TestForeignTrackingDoesNotChangeHover(t *testing.T) {
	onMain(func() {
		loadAppKit()
		before := testInputFilter
		defer func() { testInputFilter = before }()
		testInputFilter = (&scriptedInputs{}).contains
		native := objc.ID(classView).Send(selNew)
		defer native.Send(selRelease)
		s := &surface{inside: make(map[*view]struct{})}
		v := &view{native: native, surface: s}
		views[native] = v
		defer delete(views, native)
		tracking := func(kind uint) objc.ID {
			return objc.ID(apple.Class("NSEvent")).Send(
				sel("enterExitEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:trackingNumber:userData:"),
				kind, apple.CGPoint{}, uint(0), float64(12), 0, objc.ID(0), 0, 0, uintptr(0))
		}
		mouseEntered(native, 0, tracking(8))
		if v.over || len(s.inside) != 0 {
			t.Error("foreign tracking entered the scene")
		}
		mouseEntered(native, 0, 0)
		if !v.over || len(s.inside) != 1 {
			t.Error("the host could not hover its view")
		}
		mouseExited(native, 0, tracking(9))
		if !v.over || len(s.inside) != 1 {
			t.Error("foreign tracking removed the host's hover")
		}
		mouseExited(native, 0, 0)
		if v.over || len(s.inside) != 0 {
			t.Error("the host could not leave its view")
		}
	})
}

// A scripted key carries the key code and repeat flag it was made with on both Apple silicon and Intel, whose ABIs pass the selector's last two arguments differently: the keymap and Tab read the key code, not the characters.
func TestKeyEventCarriesItsKeyCode(t *testing.T) {
	onMain(func() {
		loadAppKit()
		for _, c := range []struct {
			kind   uint
			code   uint16
			repeat bool
		}{{evKeyDown, 7, false}, {evKeyUp, 48, false}, {evKeyDown, 49, true}, {evKeyDown, 0xffff, true}} {
			event := apple.KeyEvent(c.kind, flagShift, 12, 3, "X", "x", c.repeat, c.code)
			if event == 0 {
				t.Fatalf("AppKit made no key event for key %d", c.code)
			}
			if got := objc.Send[uint16](event, sel("keyCode")); got != c.code {
				t.Errorf("key %d: the event's key code is %d", c.code, got)
			}
			if got := objc.Send[bool](event, sel("isARepeat")); got != c.repeat {
				t.Errorf("key %d: isARepeat is %v, want %v", c.code, got, c.repeat)
			}
			if got := objc.Send[uint](event, sel("type")); got != c.kind {
				t.Errorf("key %d: the event's type is %d, want %d", c.code, got, c.kind)
			}
			if got := objc.Send[int](event, sel("windowNumber")); got != 3 {
				t.Errorf("key %d: the event's window is %d, want 3", c.code, got)
			}
			if chars, ignoring := apple.GoString(event.Send(sel("characters"))), apple.GoString(event.Send(sel("charactersIgnoringModifiers"))); chars != "X" || ignoring != "x" {
				t.Errorf("key %d: the event types %q, %q without modifiers", c.code, chars, ignoring)
			}
		}
	})
}
