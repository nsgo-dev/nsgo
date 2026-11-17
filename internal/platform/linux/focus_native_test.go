//go:build linux && !android

package linux

import (
	"fmt"
	"testing"

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
	w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+f"})
	waitForTitle(t, w, title+"|field|0")
	w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+b"})
	waitForTitle(t, w, title+"|button|0")
	var button *view
	w.onLoop(t, func() {
		button = w.focusedView()
		if button == nil || button.props.TestID != "button" || gtkWindowGetFocus(w.toplevel()) != button.widget {
			t.Error("Ref.Focus did not grab the button's GTK widget")
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
	waitForTitle(t, w, title+"|button|2")
	w.onLoop(t, func() {
		if button.style.Background != driver.RGB(0x123456) {
			t.Errorf("focus-visible background = %#x", button.style.Background)
		}
	})
	w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+p"})
	w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+n"})
	w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+m"})
	waitForTitle(t, w, title+"|button-checked|2")
	w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: "mod+v"})
	waitForTitle(t, w, title+"|explicit|2")
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
	})
}
