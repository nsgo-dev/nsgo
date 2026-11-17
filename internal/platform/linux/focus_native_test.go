//go:build linux && !android

package linux

import (
	"fmt"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func programmaticFocusApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		var button, plain, explicit, field core.Ref
		focused, pressed := core.NewSignal(cx, "ready"), core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%s|%d", title, focused.Get(), pressed.Get())) })
		cx.OnKeyWith("mod+f", core.KeyOptions{When: core.KeyAlways}, field.Focus)
		cx.OnKeyWith("mod+b", core.KeyOptions{When: core.KeyAlways}, button.Focus)
		cx.OnKeyWith("mod+p", core.KeyOptions{When: core.KeyAlways}, plain.Focus)
		cx.OnKeyWith("mod+v", core.KeyOptions{When: core.KeyAlways}, explicit.Focus)
		cx.OnKeyWith("mod+n", core.KeyOptions{When: core.KeyAlways}, field.Blur)
		cx.OnKeyWith("mod+m", core.KeyOptions{When: core.KeyAlways}, func() { focused.Set(focused.Get() + "-checked") })
		cx.OnKeyWith("mod+q", core.KeyOptions{When: core.KeyAlways}, cx.Quit)
		return core.View{Style: core.Style{Width: core.Pt(200), Height: core.Pt(200)}, Children: []core.Node{
			core.Input{TestID: "field", Ref: &field, Style: core.Style{Height: core.Pt(30)},
				OnFocus: func() { focused.Set("field") }},
			core.View{TestID: "button", Ref: &button, Style: core.Style{Width: core.Pt(100), Height: core.Pt(40),
				Background: core.RGB(0xcc0000), FocusVisible: &core.Style{Background: core.RGB(0x123456)}},
				OnFocus: func() { focused.Set("button") }, OnBlur: func() { focused.Set("blur") },
				OnPress: func(core.PressEvent) { pressed.Update(func(n int) int { return n + 1 }) }},
			core.View{TestID: "explicit", Ref: &explicit, Style: core.Style{Height: core.Pt(30)},
				Focusable: func() bool { return true }, OnFocus: func() { focused.Set("explicit") }},
			core.View{TestID: "plain", Ref: &plain, Style: core.Style{Height: core.Pt(30)}},
			core.View{TestID: "trigger", NoFocus: true, Style: core.Style{Height: core.Pt(30)},
				OnPress: func(core.PressEvent) { button.Focus() }},
		}}
	}
}

func TestRefFocusUsesNativeKeyboardStops(t *testing.T) {
	const title = "nsgo-ref-focus"
	w := startTestWindow(t, programmaticFocusApp(title), Options{Title: title, Width: 200, Height: 200})
	waitForTitle(t, w, title+"|ready|0")
	// The platform reports a key binding by its ID, as it does a matched press.
	key := func(name string) {
		t.Helper()
		w.onLoop(t, func() {
			for _, b := range w.keys {
				if b.Key == name {
					w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Count: b.ID})
					return
				}
			}
			t.Errorf("no binding of %s", name)
		})
	}
	key("mod+f")
	waitForTitle(t, w, title+"|field|0")
	key("mod+b")
	waitForTitle(t, w, title+"|button|0")
	var button *view
	w.onLoop(t, func() {
		button = w.focusedView()
		if button == nil || button.props.TestID != "button" || gtkWindowGetFocus(w.toplevel()) != button.widget {
			t.Error("Ref.Focus did not grab the button's GTK widget")
		}
		// A click on empty space or on a view that is no stop leaves a focused control focused, in GTK too, and Return still presses it.
		for _, at := range []driver.Point{{X: 190, Y: 190}, {X: 50, Y: 115}} {
			w.pointerDown(at, 1, 0, 0, 1)
			w.pointerUp(at)
			if w.focusedView() != button || gtkWindowGetFocus(w.toplevel()) != button.widget {
				t.Errorf("a click at %v dropped the focused button", at)
			}
		}
		if !w.controlKey(gdkReturn, 0, false) {
			t.Error("Return did not press the button after the clicks")
		}
		// The keymap has Return before the focused button: a binding that yields it to the button leaves it, one that does not takes it.
		keys := w.keys
		w.keys = []driver.KeyBinding{{Key: "enter", Yields: driver.KeyUsePress}}
		if w.shortcut(gdkReturn, 0, false) {
			t.Error("a binding that yields Return to a focused button took it")
		}
		w.keys = []driver.KeyBinding{{Key: "enter"}}
		if !w.shortcut(gdkReturn, 0, false) {
			t.Error("a binding that does not yield Return left it to the focused button")
		}
		w.keys = keys
		for _, key := range []uint32{gdkSpace, gdkReturn} {
			keyPressedCallback(0, key, key, 0, w.handle)
			// Repeated keydowns must not press again.
			keyPressedCallback(0, key, key, 0, w.handle)
			keyReleasedCallback(0, key, key, 0, w.handle)
		}
	})
	waitForTitle(t, w, title+"|button|3")
	w.onLoop(t, func() {
		if button.style.Background != driver.RGB(0x123456) {
			t.Errorf("focus-visible background = %#x", button.style.Background)
		}
	})
	key("mod+p")
	key("mod+n")
	key("mod+m")
	waitForTitle(t, w, title+"|button-checked|3")
	key("mod+v")
	waitForTitle(t, w, title+"|explicit|3")
	w.onLoop(t, func() {
		v := w.focusedView()
		if v == nil || v.props.TestID != "explicit" || gtkWindowGetFocus(w.toplevel()) != v.widget {
			t.Error("Ref.Focus did not focus the explicit stop")
		}
		if !w.tab(false) || w.focusedView() == nil || w.focusedView().props.TestID != "field" {
			t.Error("Tab did not wrap to the input")
		}
		if !w.tab(true) || w.focusedView() != v {
			t.Error("Shift+Tab did not return to the explicit stop")
		}
		// A click outside a focused field still blurs it.
		if !w.tab(false) || w.focusedView() == nil || w.focusedView().props.TestID != "field" {
			t.Error("Tab did not focus the input")
			return
		}
		w.pointerDown(driver.Point{X: 190, Y: 190}, 1, 0, 0, 1)
		w.pointerUp(driver.Point{X: 190, Y: 190})
		if w.focusedView() != nil || gtkWindowGetFocus(w.toplevel()) != 0 {
			t.Error("a click on empty space left the input focused")
		}
	})
}

func TestEmbeddedClickKeepsControlFocus(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	host := startTestWindow(t, func(*core.Scope) core.Node { return core.View{} }, Options{Width: 200, Height: 300})
	var embedded *View
	host.onLoop(t, func() {
		var err error
		embedded, err = NewView(func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Width: core.Pt(160), Height: core.Pt(100)}, Children: []core.Node{
				core.View{TestID: "button", Style: core.Style{Width: core.Pt(60), Height: core.Pt(30)}, OnPress: func(core.PressEvent) {}},
			}}
		}, Options{})
		if err != nil {
			t.Error(err)
			return
		}
		gtkWidgetSetSizeRequest(embedded.Handle(), 160, 100)
		gtkBoxAppend(host.box, embedded.Handle())
	})
	if embedded == nil {
		t.Fatal("the embedded view did not open")
	}
	surface := embedded.Handle()
	t.Cleanup(func() {
		host.onLoop(t, func() {
			embedded.Close()
			gtkWidgetUnparent(surface)
		})
	})
	var button *view
	for deadline := time.Now().Add(5 * time.Second); button == nil; time.Sleep(10 * time.Millisecond) {
		host.onLoop(t, func() {
			for _, v := range embedded.w.views {
				if v != nil && v.props.TestID == "button" && v.widget != 0 {
					button = v
				}
			}
		})
		if button == nil && time.Now().After(deadline) {
			t.Fatal("the embedded app did not mount")
		}
	}
	host.onLoop(t, func() {
		w := embedded.w
		click := func() {
			gesture := gtkGestureClickNew()
			clickPressedCallback(gesture, 1, 150, 90, w.handle)
			gObjectUnref(gesture)
		}
		w.focusView(button)
		if w.focusedView() != button || gtkWindowGetFocus(w.toplevel()) != button.widget {
			t.Error("focusView did not focus the embedded button")
			return
		}
		// The press on empty space leaves the focused control's GTK focus alone instead of moving it to the surface.
		click()
		if w.focusedView() != button || gtkWindowGetFocus(w.toplevel()) != button.widget {
			t.Error("a click on empty space blurred the embedded button")
		}
		// With nothing focused, the press still gives the surface the keys.
		w.focusEntry(nil)
		click()
		if gtkWindowGetFocus(w.toplevel()) != surface {
			t.Error("a click did not focus the embedded surface")
		}
	})
}

func TestControlFocusReturnsAfterModal(t *testing.T) {
	const title = "nsgo-modal-focus"
	var open *core.Signal[bool]
	var root *core.Scope
	var button core.Ref
	app := func(cx *core.Scope) core.Node {
		root = cx
		open = core.NewSignal(cx, false)
		cx.SetTitle(title)
		cx.OnKeyWith("mod+b", core.KeyOptions{When: core.KeyAlways}, button.Focus)
		return core.View{Style: core.Style{Width: core.Pt(240), Height: core.Pt(180)}, Children: []core.Node{
			core.View{TestID: "opener", Ref: &button, Style: core.Style{Width: core.Pt(120), Height: core.Pt(40)},
				Focusable: func() bool { return true },
				OnFocus:   func() { cx.SetTitle(title + "|focused") }},
			core.Show{When: open.Get, Then: func(*core.Scope) core.Node {
				return core.Modal{Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(120), Height: core.Pt(60)}}}}
			}},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 240, Height: 180})
	waitForTitle(t, w, title)
	w.onLoop(t, func() {
		for _, b := range w.keys {
			if b.Key == "mod+b" {
				w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Count: b.ID})
				return
			}
		}
		t.Fatal("no focus binding")
	})
	waitForTitle(t, w, title+"|focused")
	root.Post(func() { open.Set(true) })
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var covered bool
		w.onLoop(t, func() {
			covered = w.controlFocus == nil && w.restoreControl != nil && w.restoreControl.props.TestID == "opener"
		})
		if covered {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	w.onLoop(t, func() {
		if w.controlFocus != nil || w.restoreControl == nil {
			t.Fatal("opening a modal did not retain the opener")
		}
	})
	root.Post(func() { open.Set(false) })
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var restored bool
		w.onLoop(t, func() { restored = w.controlFocus != nil && w.controlFocus.props.TestID == "opener" })
		if restored {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the opener did not regain focus after the modal closed")
}
