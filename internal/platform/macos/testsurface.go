//go:build darwin && !ios && (nsgotest || nsgoautomation)

package macos

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// A surface acting as the user does, with events posted to the application's queue, for the test host (testhost.go) and for nsgo dev's automation (automation.go), whose live surfaces are the running app's windows, with no test's options: test is nil.

// onMainSync runs fn on the main thread and waits for it. It goes through
// the run loop in its common modes, as batches do, rather than the main
// queue: a context menu tracks in a loop of its own inside a main-queue
// block, which holds the queue until the menu closes.
func onMainSync(fn func()) {
	done := make(chan struct{})
	callsMu.Lock()
	calls = append(calls, func() {
		defer close(done)
		fn()
	})
	callsMu.Unlock()
	if caller == 0 {
		callerOnce.Do(func() {
			class := apple.RegisterClass("NsgoTestCaller", apple.Class("NSObject"), nil, []objc.MethodDef{{
				Cmd: sel("call"),
				Fn: func(objc.ID, objc.SEL) {
					callsMu.Lock()
					fns := calls
					calls = nil
					callsMu.Unlock()
					for _, fn := range fns {
						fn()
					}
				},
			}})
			caller = objc.ID(class).Send(selNew)
		})
	}
	caller.Send(sel("performSelectorOnMainThread:withObject:waitUntilDone:"), sel("call"), objc.ID(0), false)
	<-done
}

var (
	caller     objc.ID
	callerOnce sync.Once
	callsMu    sync.Mutex
	calls      []func()
)

// testSurface acts on s: for a test, the main window its app opened, else its own surface, own, which shows no view; for nsgo dev's automation, the running app's surface it was made for.
type testSurface struct {
	s       *surface
	own     *surface
	test    *testWindow
	hovered []objc.ID
	clicks  testhost.Clicks
}

// follow has a test's surface act on the main window its app opened, while one is open, as the test's model follows it (testhost.WindowHost), else on its own surface.
func (t *testSurface) follow() {
	if t.test == nil {
		return
	}
	if main := t.test.main.Load(); main != nil {
		t.s = main
	} else {
		t.s = t.own
	}
}

// windowPoint is p, in the surface's points from its top left, in the
// window's base coordinates.
func (t *testSurface) windowPoint(p driver.Point) apple.CGPoint {
	return objc.Send[apple.CGPoint](t.s.native, sel("convertPoint:toView:"), apple.CGPoint{X: float64(p.X), Y: float64(p.Y)}, objc.ID(0))
}

// mouse posts a mouse event of kind at p to the application's queue.
func (t *testSurface) mouse(kind uint, p driver.Point, clicks int) {
	t.mouseWith(kind, p, clicks, driver.Modifiers{})
}

func (t *testSurface) mouseWith(kind uint, p driver.Point, clicks int, mods driver.Modifiers) {
	var flags uint
	if mods.Control {
		flags |= flagControl
	}
	if mods.Alt {
		flags |= flagOption
	}
	if mods.Shift {
		flags |= flagShift
	}
	if mods.Platform {
		flags |= flagCommand
	}
	onMainSync(func() {
		if t.s.closed {
			// What it did closed the window, as a key or a press on Close does: its next event has nowhere to go.
			return
		}
		w := t.s.window
		if t.s.testPointer != nil {
			*t.s.testPointer = t.windowPoint(p)
		}
		ev := objc.ID(apple.Class("NSEvent")).Send(
			sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
			kind, t.windowPoint(p), flags, uptime(), objc.Send[int](w, sel("windowNumber")), objc.ID(0), 0, clicks, float32(1))
		rememberTestInput(ev)
		nsApp().Send(sel("postEvent:atStart:"), ev, false)
	})
}

func (t *testSurface) Press(at driver.Point, hold time.Duration, mods driver.Modifiers) error {
	t.follow()
	interval := time.Duration(objc.Send[float64](objc.ID(apple.Class("NSEvent")), sel("doubleClickInterval")) * float64(time.Second))
	clicks := t.clicks.Press(at, time.Now(), interval)
	t.mouseWith(evMoved, at, 0, mods)
	t.mouseWith(evLeftDown, at, clicks, mods)
	if hold > 0 {
		time.Sleep(hold + 50*time.Millisecond)
	}
	t.mouseWith(evLeftUp, at, clicks, mods)
	return nil
}

func (t *testSurface) RightClick(at driver.Point) error {
	t.follow()
	t.mouse(evRightDown, at, 1)
	t.mouse(evRightUp, at, 1)
	return nil
}

// Drag rests where it lands, then moves 10 points a frame, as a hand.
func (t *testSurface) Drag(path []driver.Point) error {
	t.follow()
	if len(path) == 0 {
		return errors.New("an empty drag")
	}
	t.mouse(evMoved, path[0], 0)
	t.mouse(evLeftDown, path[0], 1)
	time.Sleep(100 * time.Millisecond)
	at := path[0]
	for _, to := range path[1:] {
		for _, p := range steps(at, to, 10) {
			t.mouse(evLeftDragged, p, 1)
			time.Sleep(16 * time.Millisecond)
		}
		at = to
	}
	t.mouse(evLeftUp, at, 1)
	return nil
}

// steps are the points from a (excluded) to b, at most step apart.
func steps(a, b driver.Point, step driver.Points) []driver.Point {
	dx, dy := b.X-a.X, b.Y-a.Y
	n := int(max(abs(dx), abs(dy))/step) + 1
	out := make([]driver.Point, n)
	for i := range out {
		f := driver.Points(i+1) / driver.Points(n)
		out[i] = driver.Point{X: a.X + dx*f, Y: a.Y + dy*f}
	}
	return out
}

var (
	cgEventCreateScrollWheelEvent2 func(source uintptr, units uint32, count uint32, w1, w2, w3 int32) uintptr
	cgEventSetIntegerValueField    func(event uintptr, field uint32, value int64)
	cgEventSetLocation             func(event uintptr, p apple.CGPoint)
	cgWindowListCreateImage        func(bounds apple.CGRect, option uint32, window uint32, imageOption uint32) uintptr
	screenshotFallbackOnce         sync.Once
)

func loadCG() {
	if cgEventSetLocation != nil {
		return
	}
	cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&cgEventCreateScrollWheelEvent2, cg, "CGEventCreateScrollWheelEvent2")
	purego.RegisterLibFunc(&cgEventSetIntegerValueField, cg, "CGEventSetIntegerValueField")
	purego.RegisterLibFunc(&cgEventSetLocation, cg, "CGEventSetLocation")
	// Deprecated since macOS 14; a system without it captures through AppKit instead of failing to load.
	if _, err := purego.Dlsym(cg, "CGWindowListCreateImage"); err == nil {
		purego.RegisterLibFunc(&cgWindowListCreateImage, cg, "CGWindowListCreateImage")
	}
}

// Scroll is a trackpad gesture: it begins, moves 20 points at most every
// 16 ms, and ends without momentum.
func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	t.follow()
	loadCG()
	const pixel, scrollPhase, phaseBegan, phaseChanged, phaseEnded = 0, 99, 1, 2, 4
	// A trackpad's deltas are continuous, in points.
	const continuous = 88
	n := int(max(abs(delta.X), abs(delta.Y))/20) + 1
	// The gesture finds its view with AppKit's geometry, as the window's monitor has it do. A trackpad scrolls under the pointer, so the hover pick after the scroll is made there too.
	onMainSync(func() {
		if t.s.testPointer != nil {
			*t.s.testPointer = t.windowPoint(at)
		}
		t.s.commitPending()
	})
	send := func(phase int64, dx, dy driver.Points) {
		onMainSync(func() {
			// The wheel's deltas are the content's way: positive shows what
			// is above or left.
			ev := cgEventCreateScrollWheelEvent2(0, pixel, 2, int32(-dy), int32(-dx), 0)
			defer apple.Release(ev)
			cgEventSetIntegerValueField(ev, scrollPhase, phase)
			cgEventSetIntegerValueField(ev, continuous, 1)
			cgEventSetLocation(ev, t.screenPoint(at))
			ns := objc.ID(apple.Class("NSEvent")).Send(sel("eventWithCGEvent:"), ev)
			rememberTestInput(ns)
			// AppKit hands wheels to the window under the system's pointer,
			// which a test leaves where the user has it: the view under at
			// takes it, and passes it up as a real one.
			if hit := t.hit(at); hit != 0 {
				hit.Send(sel("scrollWheel:"), ns)
			}
		})
		time.Sleep(16 * time.Millisecond)
	}
	send(phaseBegan, 0, 0)
	for range n {
		send(phaseChanged, delta.X/driver.Points(n), delta.Y/driver.Points(n))
	}
	send(phaseEnded, 0, 0)
	return nil
}

// screenPoint is p in the global display coordinates CGEvents use: from
// the main screen's top left.
func (t *testSurface) screenPoint(p driver.Point) apple.CGPoint {
	wp := t.windowPoint(p)
	sp := objc.Send[apple.CGPoint](t.s.window, sel("convertPointToScreen:"), wp)
	screens := objc.ID(apple.Class("NSScreen")).Send(sel("screens"))
	main := screens.Send(sel("objectAtIndex:"), uint(0))
	h := objc.Send[apple.CGRect](main, sel("frame")).Size.Height
	return apple.CGPoint{X: sp.X, Y: h - sp.Y}
}

// hit is the view at at, as the window's hit testing finds it.
func (t *testSurface) hit(at driver.Point) objc.ID {
	root := t.s.native
	wp := t.windowPoint(at)
	return root.Send(sel("hitTest:"), objc.Send[apple.CGPoint](root.Send(sel("superview")), sel("convertPoint:fromView:"), wp, objc.ID(0)))
}

// Hover moves the mouse over at: posted moves reach only the first
// responder, so the views under it hear what their tracking areas would
// report.
func (t *testSurface) Hover(at driver.Point) error {
	t.follow()
	onMainSync(func() {
		if t.s.testPointer != nil {
			*t.s.testPointer = t.windowPoint(at)
		}
		t.s.commitPending()
		var under []objc.ID
		root := t.s.native
		if at.X >= 0 && at.Y >= 0 {
			for n := t.hit(at); n != 0 && n != root; n = n.Send(selSuperview) {
				if v := views[n]; v != nil && v.native == n {
					under = append(under, n)
				}
			}
		}
		in := func(list []objc.ID, n objc.ID) bool {
			for _, o := range list {
				if o == n {
					return true
				}
			}
			return false
		}
		for _, n := range t.hovered {
			// A view the app removed since is gone with its events.
			if v := views[n]; v != nil && v.native == n && !in(under, n) {
				n.Send(sel("mouseExited:"), objc.ID(0))
			}
		}
		for i := len(under) - 1; i >= 0; i-- {
			if !in(t.hovered, under[i]) {
				under[i].Send(sel("mouseEntered:"), objc.ID(0))
			}
		}
		t.hovered = under
		if len(under) > 0 {
			ev := objc.ID(apple.Class("NSEvent")).Send(
				sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
				uint(evMoved), t.windowPoint(at), uint(0), uptime(), objc.Send[int](t.s.window, sel("windowNumber")), objc.ID(0), 0, 0, float32(0))
			rememberTestInput(ev)
			for _, n := range under {
				n.Send(sel("mouseMoved:"), ev)
			}
		}
	})
	return nil
}

// editActions are the Edit menu's items by their key, which Command
// presses.
var editActions = map[string]string{"a": "selectAll:", "c": "copy:", "v": "paste:", "x": "cut:", "z": "undo:"}

// testModifiers are the left modifier keys' codes and the flags each sets, its device-dependent one included, by the names tests press them alone with.
var testModifiers = map[string]struct {
	code  uint16
	flags uint
}{
	"shift": {56, flagShift | 0x02}, "ctrl": {59, flagControl | 0x01}, "control": {59, flagControl | 0x01},
	"alt": {58, flagOption | 0x20}, "option": {58, flagOption | 0x20},
	"mod": {55, flagCommand | 0x08}, "meta": {55, flagCommand | 0x08}, "cmd": {55, flagCommand | 0x08}, "command": {55, flagCommand | 0x08},
}

// modifier presses and releases a modifier key alone, as AppKit reports it: flagsChanged with its flags set, then cleared.
func (t *testSurface) modifier(code uint16, held, flags uint) {
	const evFlagsChanged = 12
	for _, f := range []uint{held | flags, held} {
		onMainSync(func() {
			if t.s.closed {
				// What it did closed the window, as a key or a press on Close does: its next event has nowhere to go.
				return
			}
			ev := apple.KeyEvent(evFlagsChanged, f, uptime(), objc.Send[int](t.s.window, sel("windowNumber")), "", "", false, code)
			rememberTestInput(ev)
			if objc.Send[bool](t.s.window, sel("isKeyWindow")) {
				nsApp().Send(sel("postEvent:atStart:"), ev, false)
			} else if ev = t.s.filter(ev); ev != 0 {
				if r := t.s.window.Send(sel("firstResponder")); r != 0 {
					r.Send(sel("flagsChanged:"), ev)
				}
			}
		})
	}
	time.Sleep(10 * time.Millisecond)
}

// key sends a key down and up with chars, ignoring is what it types without modifiers. A background window receives its keys directly.
func (t *testSurface) key(code uint16, flags uint, chars, ignoring string) {
	for _, kind := range []uint{evKeyDown, evKeyUp} {
		onMainSync(func() {
			if t.s.closed {
				// What it did closed the window, as a key or a press on Close does: its next event has nowhere to go.
				return
			}
			ev := apple.KeyEvent(kind, flags, uptime(), objc.Send[int](t.s.window, sel("windowNumber")), chars, ignoring, false, code)
			rememberTestInput(ev)
			if objc.Send[bool](t.s.window, sel("isKeyWindow")) {
				nsApp().Send(sel("postEvent:atStart:"), ev, false)
			} else if ev = t.s.filter(ev); ev != 0 {
				// A background app can have no key window. Its monitors still see queued keys, but AppKit never hands them to the responder: run the same filter and deliver to that responder, without activating the app.
				r := t.s.window.Send(sel("firstResponder"))
				switch {
				case kind == evKeyDown && flags&(flagCommand|flagControl) != 0 && t.keyEquivalent(ev):
					// A view or the menu bar took it.
				case code == 48 && kind == evKeyDown && (r == t.s.window || r == t.s.native || r == 0):
					t.s.tabFromSurface(flags&flagShift != 0)
				case kind == evKeyUp:
					r.Send(sel("keyUp:"), ev)
				default:
					r.Send(sel("keyDown:"), ev)
				}
			}
		})
	}
	time.Sleep(10 * time.Millisecond)
}

// keyEquivalent offers a background window's key down to its views and then the menu bar, as AppKit's own dispatch does before the responder sees the key, and reports whether one took it.
func (t *testSurface) keyEquivalent(ev objc.ID) bool {
	if objc.Send[bool](t.s.window, sel("performKeyEquivalent:"), ev) {
		return true
	}
	// An item whose action has no target, as a role's Close, goes to the key window, which a background app has none of: the menu bar would take the key and send the action nowhere.
	menu := nsApp().Send(sel("mainMenu"))
	if item := keyItem(menu, apple.GoString(ev.Send(sel("charactersIgnoringModifiers"))), objc.Send[uint](ev, sel("modifierFlags"))); item != 0 && t.performUntargeted(item) {
		return true
	}
	return objc.Send[bool](menu, sel("performKeyEquivalent:"), ev)
}

// keyItem returns the item of menu or its submenus whose key equivalent is key with the modifiers in flags, 0 for none.
func keyItem(menu objc.ID, key string, flags uint) objc.ID {
	if menu == 0 || key == "" {
		return 0
	}
	const modifiers = flagCommand | flagControl | flagOption | flagShift
	for i := range objc.Send[int](menu, sel("numberOfItems")) {
		it := menu.Send(sel("itemAtIndex:"), i)
		if sub := it.Send(sel("submenu")); sub != 0 {
			if found := keyItem(sub, key, flags); found != 0 {
				return found
			}
			continue
		}
		if strings.EqualFold(apple.GoString(it.Send(sel("keyEquivalent"))), key) && objc.Send[uint](it, sel("keyEquivalentModifierMask"))&modifiers == flags&modifiers {
			return it
		}
	}
	return 0
}

// performUntargeted sends item's action, which has no target, down the window's responder chain, as AppKit sends it down the key window's, and reports whether a responder took it: a background app has no key window for AppKit to send it to.
func (t *testSurface) performUntargeted(item objc.ID) bool {
	action := objc.Send[objc.SEL](item, sel("action"))
	if action == 0 || item.Send(sel("target")) != 0 || t.s.window == 0 {
		return false
	}
	r := t.s.window.Send(sel("firstResponder"))
	if r == 0 {
		r = t.s.window
	}
	return objc.Send[bool](r, sel("tryToPerform:with:"), action, item)
}

// activates reports whether the run opted in to activating the test app for the keyboard (nsgo test -target macos -activate). Live surfaces never activate here.
func (t *testSurface) activates() bool {
	return t.test != nil && os.Getenv("NSGO_TEST_ACTIVATE") == "1"
}

// keyboard readies the window for a key step. A live surface's window becomes key within the app, without activating it, as the test host gives a test's window as it opens. A test's window is activated only with the run's opt-in; otherwise key delivers to it in the background.
func (t *testSurface) keyboard() error {
	if t.test != nil {
		return t.PrepareKeyboard()
	}
	onMainSync(func() {
		if w := t.s.window; w != 0 && !objc.Send[bool](w, sel("isKeyWindow")) {
			w.Send(sel("makeKeyWindow"))
		}
	})
	return nil
}

// PrepareKeyboard implements testhost.KeyboardSurface. With the run's opt-in it activates the app and waits up to 2 seconds for it to be active with its window key: AppKit activates asynchronously, so the wait polls from the test goroutine rather than blocking the main thread. Without the opt-in it leaves the user's active app alone and does nothing.
func (t *testSurface) PrepareKeyboard() error {
	t.follow()
	if !t.activates() {
		return nil
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var active, key bool
		onMainSync(func() {
			active = objc.Send[bool](nsApp(), sel("isActive"))
			if !active {
				activateApp()
			}
			w := t.s.window
			if !objc.Send[bool](w, sel("isKeyWindow")) {
				w.Send(sel("makeKeyWindow"))
			}
			key = objc.Send[bool](w, sel("isKeyWindow"))
		})
		if active && key {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the macOS test app could not activate and make its window key within 2s (active=%v, key=%v); -activate needs an unlocked graphical session", active, key)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// FocusRefusal implements testhost.KeyboardSurface: after a click left a field without the focus, it names an inactive app as the cause when the run did not opt in to activating it.
func (t *testSurface) FocusRefusal() error {
	t.follow()
	if t.test == nil || t.activates() {
		return nil
	}
	var active bool
	onMainSync(func() { active = objc.Send[bool](nsApp(), sel("isActive")) })
	if active {
		return nil
	}
	return errors.New("the macOS test app is not active, and AppKit gave the clicked field no keyboard focus: rerun with -target macos -activate to let the test app take the keyboard")
}

func (t *testSurface) Type(text string) error {
	t.follow()
	if err := t.keyboard(); err != nil {
		return err
	}
	for _, r := range text {
		name := strings.ToLower(string(r))
		code, flags := keyCodes[name], uint(0)
		if r == '\n' {
			code, name = 36, "\r"
		}
		if r >= 'A' && r <= 'Z' {
			flags = flagShift
		}
		t.key(code, flags, string(r), name)
	}
	return nil
}

// Compose marks text in the first responder as an input method does,
// through NSTextInputClient: the field's editor shows it marked and
// reports it, as it does a keyboard's composition.
func (t *testSurface) Compose(text string) error {
	t.follow()
	if err := t.keyboard(); err != nil {
		return err
	}
	var err error
	onMainSync(func() {
		r := t.s.window.Send(sel("firstResponder"))
		if r == 0 || !objc.Send[bool](r, sel("respondsToSelector:"), sel("setMarkedText:selectedRange:replacementRange:")) {
			err = errors.New("no text input client has the keyboard")
			return
		}
		n := uint(len(utf16.Encode([]rune(text))))
		r.Send(sel("setMarkedText:selectedRange:replacementRange:"), apple.NSString(text), apple.NSRange{Location: n}, apple.NSRange{Location: nsNotFound})
		if text == "" {
			r.Send(sel("unmarkText"))
		}
	})
	time.Sleep(10 * time.Millisecond)
	return err
}

// Commit inserts text over what is marked, which ends the composition, as
// an input method commits.
func (t *testSurface) Commit(text string) error {
	t.follow()
	if err := t.keyboard(); err != nil {
		return err
	}
	var err error
	onMainSync(func() {
		r := t.s.window.Send(sel("firstResponder"))
		if r == 0 || !objc.Send[bool](r, sel("respondsToSelector:"), sel("insertText:replacementRange:")) {
			err = errors.New("no text input client has the keyboard")
			return
		}
		r.Send(sel("insertText:replacementRange:"), apple.NSString(text), apple.NSRange{Location: nsNotFound})
	})
	time.Sleep(10 * time.Millisecond)
	return err
}

// nsNotFound is NSNotFound, no range to replace: the marked text's.
const nsNotFound = uint(1<<63 - 1)

func (t *testSurface) Key(key string) error {
	t.follow()
	if err := t.keyboard(); err != nil {
		return err
	}
	parts := strings.Split(strings.ToLower(key), "+")
	name := parts[len(parts)-1]
	var flags uint
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod", "cmd", "meta", "command":
			flags |= flagCommand
		case "ctrl", "control":
			flags |= flagControl
		case "alt", "option":
			flags |= flagOption
		case "shift":
			flags |= flagShift
		default:
			return fmt.Errorf("unknown modifier %q in %q", m, key)
		}
	}
	if m, ok := testModifiers[name]; ok {
		t.modifier(m.code, flags, m.flags)
		return nil
	}
	code, ok := keyCodes[name]
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	if action := editActions[name]; flags == flagCommand && action != "" {
		// The menu's key equivalents need the app active, which a test
		// leaves to the app the user works in: the field does the edit the
		// menu item would ask of it.
		onMainSync(func() {
			ev := apple.KeyEvent(evKeyDown, flags, uptime(), objc.Send[int](t.s.window, sel("windowNumber")), name, name, false, code)
			rememberTestInput(ev)
			if t.s.filter(ev) == 0 {
				return
			}
			if r := t.s.window.Send(sel("firstResponder")); r != 0 {
				r.Send(sel("tryToPerform:with:"), sel(action), objc.ID(0))
			}
		})
		return nil
	}
	chars := keyChars[name]
	if chars == "" {
		chars = name
	}
	t.key(code, flags, chars, chars)
	return nil
}

// Back is Escape, twice in a field, whose first only leaves it.
func (t *testSurface) Back() error {
	t.follow()
	var editing bool
	onMainSync(func() {
		r := t.s.window.Send(sel("firstResponder"))
		editing = r != 0 && objc.Send[bool](r, sel("isKindOfClass:"), objc.ID(apple.Class("NSText")))
	})
	if editing {
		t.Key("escape")
	}
	return t.Key("escape")
}

func (t *testSurface) Answer(id uint32, index int) error {
	t.follow()
	var err error
	onMainSync(func() {
		s := t.s
		shown, ok := s.alerts[id]
		if !ok && t.own != nil && s != t.own {
			// One the app's own surface shows, as from the app's function: each surface numbers its alerts itself, and the main window's come first.
			s = t.own
			shown, ok = s.alerts[id]
		}
		if !ok {
			err = fmt.Errorf("no alert %d shows", id)
			return
		}
		window := shown.alert.Send(sel("window"))
		if index < 0 && (s.window == 0 || s.kind == driver.SurfaceTrayIcon) {
			// The app's own alert runs modal, with no window to end a sheet of.
			nsApp().Send(sel("abortModal"))
			return
		}
		if index < 0 {
			s.window.Send(sel("endSheet:returnCode:"), window, 0)
			return
		}
		for i, action := range shown.order {
			if action == index {
				buttons := shown.alert.Send(sel("buttons"))
				buttons.Send(sel("objectAtIndex:"), uint(i)).Send(sel("performClick:"), objc.ID(0))
				return
			}
		}
		err = fmt.Errorf("the alert has no button %d", index)
	})
	return err
}

// composited is the PNG of the view's bounds as WindowServer composites its window, nil when it cannot have them. CGWindowListCreateImage asks for the frame already on screen, so it keeps the Core Animation shadows, masks, filters, presentation layers and insertion point that an AppKit cacheDisplayInRect: redraw omits. Only the surface's own window is asked for: with no window number, kCGNullWindowID would capture whatever other apps show in that rectangle. A window that is not visible on the active Space, ordered out or minimized, is not asked for, so it draws through AppKit as before; nor is an image smaller than the view's pixels kept.
func (t *testSurface) composited(bounds apple.CGRect) []byte {
	w := t.s.window
	if w == 0 || bounds.Size.Width <= 0 || bounds.Size.Height <= 0 ||
		!objc.Send[bool](w, sel("isVisible")) || !objc.Send[bool](w, sel("isOnActiveSpace")) {
		return nil
	}
	number := objc.Send[int](w, sel("windowNumber"))
	if number <= 0 {
		return nil
	}
	if loadCG(); cgWindowListCreateImage == nil {
		return nil
	}
	// screenPoint is in the global top-left coordinates CGWindowListCreateImage takes.
	origin := t.screenPoint(driver.Point{})
	const (
		windowListIncludingWindow    = 1 << 3
		windowImageBoundsIgnoreFrame = 1 << 0
		windowImageBestResolution    = 1 << 3
	)
	image := cgWindowListCreateImage(apple.Rect(origin.X, origin.Y, bounds.Size.Width, bounds.Size.Height),
		windowListIncludingWindow, uint32(number), windowImageBoundsIgnoreFrame|windowImageBestResolution)
	if image == 0 {
		return nil
	}
	defer apple.Release(image)
	scale := objc.Send[float64](w, sel("backingScaleFactor"))
	width, height := apple.ImageSize(image)
	if float64(width) < math.Floor(bounds.Size.Width*scale) || float64(height) < math.Floor(bounds.Size.Height*scale) {
		return nil
	}
	png, _ := apple.PNGImage(image)
	return png
}

func (t *testSurface) Screenshot() ([]byte, error) {
	t.follow()
	var png []byte
	onMainSync(func() {
		v := t.s.native
		if v == 0 {
			return
		}
		bounds := objc.Send[apple.CGRect](v, selBounds)
		if png = t.composited(bounds); png != nil {
			return
		}
		screenshotFallbackOnce.Do(func() {
			log.Printf("nsgo: macOS composited screenshot unavailable; falling back to cacheDisplayInRect: (shadows, animations and visual effects may be missing)")
		})
		rep := v.Send(sel("bitmapImageRepForCachingDisplayInRect:"), bounds)
		if rep == 0 {
			return
		}
		v.Send(sel("cacheDisplayInRect:toBitmapImageRep:"), bounds, rep)
		const pngType = 4
		data := rep.Send(sel("representationUsingType:properties:"), uint(pngType),
			objc.ID(apple.Class("NSDictionary")).Send(sel("dictionary")))
		if data == 0 {
			return
		}
		png = apple.Bytes(data)
	})
	if png == nil {
		return nil, errors.New("the window drew nothing")
	}
	return png, nil
}

func (t *testSurface) Sync() { t.s.engine.Sync() }

func (t *testSurface) Animating() bool {
	t.follow()
	var busy bool
	onMainSync(func() {
		// Batches waiting for a frame have not shown yet, nor scrolls
		// their offsets.
		busy = t.s.animator.Ending() || len(t.s.motions) > 0 || len(t.s.ghosts) > 0 || t.s.held() || t.s.scrollsMoving()
	})
	return busy
}

func (t *testSurface) Resize(size driver.Size) error {
	t.follow()
	onMainSync(func() {
		t.s.window.Send(sel("setContentSize:"), apple.CGSize{Width: float64(size.Width), Height: float64(size.Height)})
	})
	return nil
}

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.follow()
	if t.test == nil {
		return errors.New("a running app's window follows the system's settings")
	}
	t.test.pin.Set(&env)
	t.own.engine.Repin()
	// The windows the app opened report their last environment again, which the new pin applies over, as Repin does for the own surface.
	onMainSync(func() {
		for s := range surfaces {
			if s.test == t.test && s != t.own && s.engine != nil && !s.closed {
				s.engine.SetEnvironment(s.env)
			}
		}
	})
	return nil
}

// menuItem returns the item tagged id in menu or its submenus, and the
// menu it is in.
func menuItem(menu objc.ID, id uint32) (item, in objc.ID) {
	if menu == 0 {
		return 0, 0
	}
	n := objc.Send[int](menu, sel("numberOfItems"))
	for i := range n {
		it := menu.Send(sel("itemAtIndex:"), i)
		if objc.Send[int](it, sel("tag")) == int(id) && it.Send(sel("action")) != 0 {
			return it, menu
		}
		if sub := it.Send(sel("submenu")); sub != 0 {
			if found, at := menuItem(sub, id); found != 0 {
				return found, at
			}
		}
	}
	return 0, 0
}

// ChooseMenu implements testhost.Surface: the item of the surface's menu sends its action, as a click on it does. Each surface numbers its menu's items itself, so the menu bar must show the surface's menu, or the default one where the surface set none.
func (t *testSurface) ChooseMenu(id uint32) error {
	t.follow()
	var err error
	onMainSync(func() {
		main := objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication")).Send(sel("mainMenu"))
		if t.s.menu != 0 && main != t.s.menu {
			err = errors.New("the menu bar shows another window's menu")
			return
		}
		item, menu := menuItem(main, id)
		if item == 0 {
			err = fmt.Errorf("the main menu has no item %d", id)
			return
		}
		menu.Send(sel("performActionForItemAtIndex:"), objc.Send[int](menu, sel("indexOfItem:"), item))
	})
	return err
}

// ChooseDockMenu implements testhost.Surface: the item of the Dock menu, as the app delegate hands it to AppKit where it is the app's, sends its action, as a click on it does.
func (t *testSurface) ChooseDockMenu(id uint32) error {
	t.follow()
	var err error
	onMainSync(func() {
		menu := dockMenu.menu
		app := objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication"))
		if d, dock := app.Send(sel("delegate")), sel("applicationDockMenu:"); d != 0 && objc.Send[bool](d, sel("respondsToSelector:"), dock) {
			menu = d.Send(dock, app)
		}
		item, in := menuItem(menu, id)
		if item == 0 {
			err = fmt.Errorf("the Dock menu has no item %d", id)
			return
		}
		in.Send(sel("performActionForItemAtIndex:"), objc.Send[int](in, sel("indexOfItem:"), item))
	})
	return err
}

// ChoosePopup implements testhost.Surface: the tracking context menu's item sends its action and the menu closes, as a click on it does. For 0 the menu showing or about to show is cancelled, and it returns once AppKit's tracking loop has ended, so the next input reaches a window; with none showing it does nothing.
func (t *testSurface) ChoosePopup(id uint32) error {
	t.follow()
	if id == 0 {
		tracking := true
		onMainSync(func() {
			t.s.closePopup()
			tracking = t.s.tracking
		})
		for deadline := time.Now().Add(3 * time.Second); tracking && time.Now().Before(deadline); {
			time.Sleep(10 * time.Millisecond)
			onMainSync(func() { tracking = t.s.tracking })
		}
		if tracking {
			return errors.New("the context menu kept tracking after it was cancelled")
		}
		return nil
	}
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		shown := false
		onMainSync(func() {
			s := t.s
			if shown = s.popup != 0 && s.tracking; !shown {
				return
			}
			if id != 0 {
				if item, menu := menuItem(s.popup, id); item != 0 {
					menu.Send(sel("performActionForItemAtIndex:"), objc.Send[int](menu, sel("indexOfItem:"), item))
				} else {
					err = fmt.Errorf("the context menu has no item %d", id)
				}
			}
			s.closePopup()
		})
		if shown || time.Now().After(deadline) {
			if !shown {
				return fmt.Errorf("no context menu shows")
			}
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}
