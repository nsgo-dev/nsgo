//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// List anchoring can precede the new content size in a mount batch. The browser must see that size before it clamps the relative scroll.
func TestAnchoringScrollWaitsForTheBatchContentSize(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("run with nsgo test -target web")
	}
	mount := doc.Call("createElement", "div")
	mount.Set("id", "nsgo-scroll-order")
	setStyle(mount, map[string]string{"width": "200px", "height": "100px"})
	doc.Get("body").Call("appendChild", mount)
	defer mount.Call("remove")
	s := open(func(*core.Scope) core.Node {
		return core.Scroll{TestID: "scroll", Style: core.Style{Width: core.Pt(200), Height: core.Pt(100)},
			Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(200)}}}}
	}, Options{Mount: "#nsgo-scroll-order"}, nil)
	defer s.engine.Close()
	var v *view
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for _, candidate := range s.views {
			if candidate != nil && candidate.kind == driver.KindScroll && candidate.scroll.Height == 200 {
				v = candidate
				break
			}
		}
		if v != nil {
			break
		}
	}
	if v == nil {
		t.Fatal("the scroll view never mounted")
	}
	apply := func(items ...driver.MountItem) { s.mount(&driver.MountBatch{Items: items}) }
	apply(driver.MountItem{Kind: driver.ItemScrollTo, ID: v.id, Point: driver.Point{Y: 100}})
	apply(
		driver.MountItem{Kind: driver.ItemScrollBy, ID: v.id, Point: driver.Point{Y: 200}},
		driver.MountItem{Kind: driver.ItemScrollContent, ID: v.id, Size: driver.Size{Width: 200, Height: 400}},
	)
	if got := v.offset().Y; got != 300 {
		t.Fatalf("anchored offset = %v, want 300", got)
	}
	// Absolute and relative commands still execute in protocol order.
	apply(
		driver.MountItem{Kind: driver.ItemScrollTo, ID: v.id, Point: driver.Point{Y: 40}},
		driver.MountItem{Kind: driver.ItemScrollBy, ID: v.id, Point: driver.Point{Y: 20}},
		driver.MountItem{Kind: driver.ItemScrollTo, ID: v.id, Point: driver.Point{Y: 120}},
		driver.MountItem{Kind: driver.ItemScrollBy, ID: v.id, Point: driver.Point{Y: 30}},
	)
	if got := v.offset().Y; got != 150 {
		t.Fatalf("ordered offset = %v, want 150", got)
	}
}
