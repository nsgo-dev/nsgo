//go:build js && wasm

package web

import (
	"math"
	"syscall/js"
	"testing"
)

func TestPointerCoordinatesUndoNestedCSS(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("run with nsgo test -target web")
	}
	root := doc.Call("createElement", "div")
	root.Get("style").Set("cssText", "position:fixed;left:0;top:0;width:500px;height:500px")
	doc.Get("body").Call("appendChild", root)
	defer root.Call("remove")
	parent := doc.Call("createElement", "div")
	parent.Get("style").Set("cssText", "position:absolute;left:100px;top:100px;width:40px;height:40px;transform:rotate(90deg) scale(2)")
	root.Call("appendChild", parent)
	child := doc.Call("createElement", "div")
	child.Get("style").Set("cssText", "position:absolute;left:5px;top:5px;width:20px;height:20px;transform:translateY(-100px)")
	parent.Call("appendChild", child)
	// Untransformed views keep fractional positions, which offsetLeft rounds.
	plain := doc.Call("createElement", "div")
	plain.Get("style").Set("cssText", "position:absolute;left:200.5px;top:300.25px;width:20px;height:20px")
	root.Call("appendChild", plain)
	if x, y := clientPoint(plain, 210.5, 310.25); math.Abs(x-10) > .001 || math.Abs(y-10) > .001 {
		t.Errorf("untransformed local = (%g,%g), want (10,10)", x, y)
	}
	if hit := doc.Call("elementFromPoint", 330, 110); !hit.Equal(child) {
		t.Fatal("browser did not pick drawn child")
	}
	x, y := clientPoint(child, 330, 110)
	if math.Abs(x-10) > .001 || math.Abs(y-10) > .001 {
		t.Fatalf("local = (%g,%g), want (10,10)", x, y)
	}
	// Individual properties are used by NSGo's keyframes as well as the transform property. They must be composed in CSS's order.
	child.Get("style").Set("translate", "10px 0px")
	x, y = clientPoint(child, 330, 130)
	if math.Abs(x-10) > .001 || math.Abs(y-10) > .001 {
		t.Errorf("individual translate local = (%g,%g)", x, y)
	}
}
