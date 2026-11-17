//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

func TestVisibleLabelARIAReferences(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("run with nsgo test -target web")
	}
	mount := doc.Call("createElement", "div")
	mount.Set("id", "nsgo-label-relations")
	doc.Get("body").Call("appendChild", mount)
	defer mount.Call("remove")
	label, extra, field := new(core.Ref), new(core.Ref), new(core.Ref)
	var scope *core.Scope
	var show *core.Signal[bool]
	s := open(func(cx *core.Scope) core.Node {
		scope = cx
		show = core.NewSignal(cx, true)
		return core.View{Children: []core.Node{
			core.Text{TestID: "label", Ref: label, Content: "Email", LabelFor: field},
			core.View{Ref: extra, Children: []core.Node{core.Text{Content: "Required"}}},
			core.Show{When: show.Get, Then: func(*core.Scope) core.Node {
				return core.Input{Ref: field, TestID: "field", LabelledBy: []*core.Ref{label, extra}}
			}},
		}}
	}, Options{Mount: "#nsgo-label-relations"}, nil)
	defer s.engine.Close()
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
	wait("label relations", func() bool {
		return find("field").Truthy() && find("field").Call("hasAttribute", "aria-labelledby").Bool()
	})
	input, source := find("field"), find("label")
	ids := strings.Fields(input.Call("getAttribute", "aria-labelledby").String())
	if len(ids) != 2 || !doc.Call("getElementById", ids[0]).Equal(source) || !doc.Call("getElementById", ids[1]).Truthy() {
		t.Fatalf("invalid DOM label references: %v", ids)
	}
	if input.Get("tagName").String() != "INPUT" || input.Call("getAttribute", "aria-label").String() != "Email Required" {
		t.Fatal("relations did not reach actual input")
	}
	if source.Call("hasAttribute", "tabindex").Bool() || source.Call("getAttribute", "role").String() == "button" {
		t.Fatal("label acquired a Tab stop or button role")
	}
	scope.Post(func() { show.Set(false) })
	wait("target removal", func() bool { return !find("field").Truthy() })
	scope.Post(func() { show.Set(true) })
	wait("target remount", func() bool {
		return find("field").Truthy() && find("field").Call("getAttribute", "aria-labelledby").String() == strings.Join(ids, " ")
	})
}
