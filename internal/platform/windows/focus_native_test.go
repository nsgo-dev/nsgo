//go:build windows

package windows

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
	hwnd := startApp(t, title, programmaticFocusApp(title), 200, 200)
	waitForTitle(t, hwnd, title+"|ready|0", 5*time.Second)
	// The platform reports a key binding by its ID, as it does a matched press.
	key := func(w *window, name string) {
		for _, b := range w.keys {
			if b.Key == name {
				w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Count: b.ID})
				return
			}
		}
		t.Errorf("no binding of %s", name)
	}
	onMain(t, hwnd, func(w *window) { key(w, "mod+f") })
	waitForTitle(t, hwnd, title+"|field|0", 5*time.Second)
	onMain(t, hwnd, func(w *window) { key(w, "mod+b") })
	waitForTitle(t, hwnd, title+"|button|0", 5*time.Second)
	onMain(t, hwnd, func(w *window) {
		v := w.focusedView()
		if v == nil || v.props.TestID != "button" || w.focusedField() != nil {
			t.Error("Ref.Focus did not move the surface's keyboard focus to the button")
		}
		// The keymap has Return before the focused button: a binding that yields it to the button leaves it, one that does not takes it.
		keys := w.keys
		w.keys = []driver.KeyBinding{{Key: "enter", Yields: driver.KeyUsePress}}
		if w.shortcut(vkReturn, 1) {
			t.Error("a binding that yields Return to a focused button took it")
		}
		w.keys = []driver.KeyBinding{{Key: "enter"}}
		if !w.shortcut(vkReturn, 1) {
			t.Error("a binding that does not yield Return left it to the focused button")
		}
		w.keys = keys
		for _, key := range []uintptr{vkSpace, vkReturn} {
			postMessageW.Call(hwnd, wmKeyDown, key, 1)
			postMessageW.Call(hwnd, wmKeyDown, key, 1<<30|1)
			postMessageW.Call(hwnd, wmKeyUp, key, 1)
		}
	})
	waitForTitle(t, hwnd, title+"|button|2", 5*time.Second)
	onMain(t, hwnd, func(w *window) {
		if v := w.focusedView(); v == nil || v.style.Background != driver.RGB(0x123456) {
			t.Error("the focused button did not receive its focus-visible style")
		}
		key(w, "mod+p")
		key(w, "mod+n")
		key(w, "mod+m")
	})
	waitForTitle(t, hwnd, title+"|button-checked|2", 5*time.Second)
	onMain(t, hwnd, func(w *window) { key(w, "mod+v") })
	waitForTitle(t, hwnd, title+"|explicit|2", 5*time.Second)
	onMain(t, hwnd, func(w *window) {
		v := w.focusedView()
		if v == nil || v.props.TestID != "explicit" || !w.tab(false) || w.focusedView() == nil || w.focusedView().props.TestID != "field" {
			t.Error("Tab did not wrap from the explicit stop to the input")
		}
		if !w.tab(true) || w.focusedView() != v {
			t.Error("Shift+Tab did not return to the explicit stop")
		}
	})
}
