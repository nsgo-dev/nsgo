package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestTopmostOverlayStaysAboveLaterModals(t *testing.T) {
	var later, cancels *Signal[bool]
	var where *Signal[int]
	var log []string
	var held *Holder
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		later, cancels = NewSignal(cx, false), NewSignal(cx, false)
		where = NewSignal(cx, 0)
		held = NewHolder(cx, func(cx *Scope) Node {
			return Overlay{TestID: "moved", Style: Style{Width: Pt(10), Height: Pt(10)}}
		})
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			Overlay{Topmost: true, TestID: "top", PointerEvents: PointerEventsBoxNone, Style: Style{Width: Pct(100), Height: Pct(100)}},
			Show{When: later.Get, Then: func(cx *Scope) Node {
				return Group{
					Modal{TestID: "modal", OnRequestClose: func(string) { log = append(log, "modal") }, Children: []Node{
						Show{When: func() bool { return where.Get() == 1 }, Then: func(cx *Scope) Node { return held.Place() }},
					}},
					Overlay{TestID: "plain", Style: Style{Width: Pt(10), Height: Pt(10)}},
				}
			}},
			Show{When: cancels.Get, Then: func(cx *Scope) Node {
				return Overlay{Topmost: true, TestID: "second", OnCancel: func() { log = append(log, "second") }}
			}},
			Show{When: func() bool { return where.Get() == 2 }, Then: func(cx *Scope) Node { return held.Place() }},
		}}
	})
	h.commit()
	later.Set(true)
	h.commit()
	// A holder's content waits at the root until a place shows it.
	moved := false
	ids := func() []string {
		var out []string
		for _, n := range h.tree.view.hosted {
			if id := n.props.TestID; id != "moved" || moved {
				out = append(out, id)
			}
		}
		return out
	}
	if got := ids(); !slices.Equal(got, []string{"modal", "plain", "top"}) {
		t.Fatalf("root hosts %q, want the Topmost overlay last", got)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if !slices.Equal(log, []string{"modal"}) {
		t.Fatalf("log = %q, want back at the modal past a Topmost overlay that does not listen", log)
	}
	cancels.Set(true)
	h.commit()
	if got := ids(); !slices.Equal(got, []string{"modal", "plain", "top", "second"}) {
		t.Fatalf("root hosts %q, want Topmost overlays in the order they came", got)
	}
	log = nil
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if !slices.Equal(log, []string{"second"}) {
		t.Fatalf("log = %q, want back at the Topmost overlay before the modal", log)
	}
	// An overlay a holder moves out of the modal goes below them too.
	where.Set(1)
	h.commit()
	if h.find("moved").hoist.host != h.find("modal") {
		t.Fatal("the holder's overlay is not in the modal")
	}
	where.Set(2)
	h.commit()
	moved = true
	if got := ids(); !slices.Equal(got, []string{"modal", "plain", "moved", "top", "second"}) {
		t.Fatalf("root hosts %q after a rehost, want the Topmost overlays last", got)
	}
}

func TestPressOutsidePassesThroughTopmostLayers(t *testing.T) {
	var log []string
	var ruler *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		ruler = NewSignal(cx, false)
		fill := Style{Top: Pt(0), Left: Pt(0), Width: Pct(100), Height: Pct(100)}
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			Overlay{TestID: "popover", Style: Style{Top: Pt(0), Left: Pt(0), Width: Pt(50), Height: Pt(30)},
				OnPressOutside: func(Point) { log = append(log, "outside") }},
			Overlay{Topmost: true, PointerEvents: PointerEventsBoxNone, Style: fill, Children: []Node{
				Canvas{Style: Style{Position: Absolute, Width: Pct(100), Height: Pct(100), PointerEvents: PointerEventsNone}},
				View{TestID: "chip", OnPress: func(PressEvent) {}, Style: Style{Position: Absolute, Left: Pt(8), Bottom: Pt(8), Width: Pt(40), Height: Pt(20)}},
			}},
			Show{When: ruler.Get, Then: func(cx *Scope) Node {
				return Overlay{Topmost: true, Style: fill}
			}},
		}}
	})
	h.commit()
	pop := h.find("popover")
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 100, Y: 100}})
	if !slices.Equal(log, []string{"outside"}) {
		t.Fatalf("log = %q, want a press through the layer to reach the popover", log)
	}
	log = nil
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 20, Y: 280}})
	if len(log) != 0 {
		t.Fatalf("log = %q, want a press on the layer's chip kept from the popover", log)
	}
	ruler.Set(true)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 100, Y: 100}})
	if len(log) != 0 {
		t.Fatalf("log = %q, want a press the ruler's layer takes kept from the popover", log)
	}
}

func TestPressOutsideAModalsPopoverHeedsTheRootsLayers(t *testing.T) {
	var log []string
	var ruler *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		ruler = NewSignal(cx, false)
		fill := Style{Top: Pt(0), Left: Pt(0), Width: Pct(100), Height: Pct(100)}
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			Modal{TestID: "modal", Children: []Node{
				Overlay{TestID: "popover", Style: Style{Top: Pt(0), Left: Pt(0), Width: Pt(50), Height: Pt(30)},
					OnPressOutside: func(Point) { log = append(log, "outside") }},
			}},
			Overlay{Topmost: true, PointerEvents: PointerEventsBoxNone, Style: fill, Children: []Node{
				View{TestID: "chip", OnPress: func(PressEvent) {}, Style: Style{Position: Absolute, Left: Pt(8), Bottom: Pt(8), Width: Pt(40), Height: Pt(20)}},
			}},
			Show{When: ruler.Get, Then: func(cx *Scope) Node {
				return Overlay{Topmost: true, Style: fill}
			}},
		}}
	})
	h.commit()
	pop := h.find("popover")
	if pop.hoist.host != h.find("modal") {
		t.Fatal("the popover is not in the modal")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 100, Y: 100}})
	if !slices.Equal(log, []string{"outside"}) {
		t.Fatalf("log = %q, want a press through the layer to reach the popover", log)
	}
	log = nil
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 20, Y: 280}})
	if len(log) != 0 {
		t.Fatalf("log = %q, want a press on the root layer's chip, above the modal, kept from the popover", log)
	}
	ruler.Set(true)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: pop.id, Point: Point{X: 100, Y: 100}})
	if len(log) != 0 {
		t.Fatalf("log = %q, want a press the root's ruler layer takes kept from the modal's popover", log)
	}
}
