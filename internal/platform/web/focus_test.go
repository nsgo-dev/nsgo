//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

func TestRefFocusUsesNativeKeyboardStops(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("no document here: run with nsgo test -target web")
	}
	mount := doc.Call("createElement", "div")
	mount.Set("id", "nsgo-ref-focus")
	setStyle(mount, map[string]string{"width": "200px", "height": "200px"})
	doc.Get("body").Call("appendChild", mount)
	defer mount.Call("remove")
	var field, button, plain, explicit core.Ref
	var scope *core.Scope
	pressed := 0
	s := open(func(cx *core.Scope) core.Node {
		scope = cx
		row := core.Style{Height: core.Pt(30)}
		return core.View{Children: []core.Node{
			core.Input{TestID: "field", Ref: &field, Style: row},
			core.View{TestID: "button", Ref: &button, Style: row,
				OnPress: func(core.PressEvent) { pressed++ },
				Class:   "focus-visible:bg-[#123456]"},
			core.View{TestID: "plain", Ref: &plain, Style: row},
			core.View{TestID: "explicit", Ref: &explicit, Style: row, Focusable: func() bool { return true }},
		}}
	}, Options{Mount: "#nsgo-ref-focus"}, nil)
	wait := func(what string, ready func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !ready() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	find := func(id string) js.Value { return mount.Call("querySelector", "[data-testid="+id+"]") }
	wait("the mounted controls", func() bool { return find("explicit").Truthy() })
	scope.Post(field.Focus)
	wait("the field's focus", func() bool { return find("field").Call("contains", doc.Get("activeElement")).Bool() })
	scope.Post(button.Focus)
	wait("the button's DOM focus and style", func() bool {
		el := find("button")
		return el.Equal(doc.Get("activeElement")) && js.Global().Call("getComputedStyle", el).Get("backgroundColor").String() == "rgb(18, 52, 86)"
	})
	for _, key := range []string{" ", "Enter"} {
		for _, repeat := range []bool{false, true} {
			event := js.Global().Get("KeyboardEvent").New("keydown", map[string]any{
				"key": key, "repeat": repeat, "bubbles": true, "cancelable": true,
			})
			find("button").Call("dispatchEvent", event)
		}
	}
	wait("one activation per key", func() bool { return pressed == 2 })
	applied := make(chan struct{})
	scope.Post(func() {
		plain.Focus()
		field.Blur()
		scope.Post(func() { close(applied) })
	})
	<-applied
	if !find("button").Equal(doc.Get("activeElement")) {
		t.Fatal("an invalid focus or blur of another view stole the button's focus")
	}
	scope.Post(explicit.Focus)
	wait("the explicit stop's focus", func() bool { return find("explicit").Equal(doc.Get("activeElement")) })
	scope.Post(explicit.Blur)
	wait("the explicit stop's blur", func() bool { return !find("explicit").Equal(doc.Get("activeElement")) })
	// This test owns the surface's engine; browser listeners last until the per-test page closes, as in the backend's other browser tests.
	s.engine.Close()
}
