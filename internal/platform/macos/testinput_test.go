//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// AppKit gates delivery with the area's activity option, before any of our callbacks run. Check the native subscriptions, including the surface's moves that pick hovers and set cursors over views with no mouse handlers of their own.
func TestPointerTrackingContinuesInBackground(t *testing.T) {
	onMain(func() {
		loadAppKit()
		s := newSurface(Options{Width: 300, Height: 200}, false)
		defer func() {
			s.close()
			delete(views, s.native)
			s.native.Send(selRelease)
		}()
		const enteredAndExited, moved, cursorUpdate = 0x01, 0x02, 0x04
		const activityMask, activeInApp, always = 0xf0, 0x40, 0x80
		areas := s.native.Send(sel("trackingAreas"))
		var rootMoves, cursorUpdates bool
		for i := uint(0); i < objc.Send[uint](areas, selCount); i++ {
			area := areas.Send(sel("objectAtIndex:"), i)
			options := objc.Send[uint](area, sel("options"))
			if options&moved != 0 {
				rootMoves = true
				if options&activityMask != always {
					t.Errorf("surface moves stop in a background app: options %#x", options)
				}
				if options&enteredAndExited != 0 {
					t.Errorf("the surface hears its own enters and exits, which no view of it follows: options %#x", options)
				}
			}
			if options&cursorUpdate != 0 {
				cursorUpdates = true
				if options&activityMask != activeInApp {
					t.Errorf("cursorUpdate needs an active-app area: options %#x", options)
				}
			}
		}
		if !rootMoves || !cursorUpdates {
			t.Errorf("surface tracks moves=%v, cursor updates=%v", rootMoves, cursorUpdates)
		}
		native := objc.ID(classView).Send(selNew)
		defer native.Send(selRelease)
		v := &view{native: native, surface: s}
		defer func() {
			if v.hover != 0 {
				v.hover.Send(selRelease)
			}
		}()
		for _, kind := range []driver.EventKind{driver.EventMouseEnter, driver.EventMouseMove, driver.EventMouseLeave} {
			v.setEvents(driver.EventMask(1) << kind)
			options := objc.Send[uint](v.hover, sel("options"))
			if options&activityMask != always || options&enteredAndExited == 0 {
				t.Errorf("%v stops entering/leaving in background: options %#x", kind, options)
			}
			if kind == driver.EventMouseMove && options&moved == 0 {
				t.Error("the move listener has no native move subscription")
			}
		}
	})
}

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
