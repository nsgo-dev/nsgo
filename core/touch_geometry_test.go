package core

import (
	"fmt"
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestTouchHandlersShareTheNativePresentationSnapshot(t *testing.T) {
	var got []string
	note := func(kind string) func(ResponderEvent) {
		return func(e ResponderEvent) { got = append(got, fmt.Sprintf("%s %v", kind, e.Position)) }
	}
	h := newHost(t, Size{Width: 400, Height: 400}, func(*Scope) Node {
		return View{TestID: "outer", OnTouchCapture: func(e TouchEvent) {
			got = append(got, fmt.Sprintf("capture %v", e.Touches[0].Position))
		}, Children: []Node{View{TestID: "inner", Style: Style{Width: Pt(50), Height: Pt(50)},
			OnTouch: func(e TouchEvent) { got = append(got, fmt.Sprintf("touch %v", e.Touches[0].Position)) },
			Responder: &Responder{Claims: []TouchClaim{{Ask: func(e ResponderEvent) bool {
				note("claim")(e)
				return true
			}}}, OnGrant: note("grant"), OnStart: note("start"), OnMove: note("move"), OnRelease: note("release")},
		}}}
	})
	h.commit()
	inner, outer := h.find("inner").id, h.find("outer").id
	ev := finger(GestureBegan, Point{X: 230, Y: 240}, 0)
	ev.TouchGeometry = &driver.TouchGeometry{Targets: map[int]NodeID{0: inner}, Positions: map[NodeID][]Point{
		inner: {{X: 10, Y: 20}}, outer: {{X: 30, Y: 40}},
	}}
	h.tree.Dispatch(ev)
	want := []string{"capture {30 40}", "touch {10 20}", "claim {10 20}", "grant {10 20}", "start {10 20}"}
	if !slices.Equal(got, want) {
		t.Fatalf("start = %v, want %v", got, want)
	}
	got = nil
	ev = finger(GestureChanged, Point{X: 250, Y: 260}, 16)
	ev.TouchGeometry = &driver.TouchGeometry{Positions: map[NodeID][]Point{
		inner: {{X: 10, Y: 20}}, outer: {{X: 50, Y: 60}},
	}}
	h.tree.Dispatch(ev)
	if want = []string{"capture {50 60}", "touch {10 20}", "move {10 20}"}; !slices.Equal(got, want) {
		t.Fatalf("after native movement = %v, want %v", got, want)
	}
	got = nil
	ev.Phase = GestureEnded
	h.tree.Dispatch(ev)
	got = nil
	ev = finger(GestureBegan, Point{X: 10, Y: 20}, 32)
	ev.TouchGeometry = &driver.TouchGeometry{Targets: map[int]NodeID{0: 0}}
	h.tree.Dispatch(ev)
	if len(got) != 0 {
		t.Fatalf("a native miss fell back to the layout hit: %v", got)
	}
}

func TestResponderUsesRemainingFingersPresentationAfterItsFingerLifts(t *testing.T) {
	var moves []ResponderEvent
	h := newHost(t, Size{Width: 400, Height: 400}, func(*Scope) Node {
		return View{Children: []Node{
			View{TestID: "owner", Style: Style{Width: Pt(50), Height: Pt(50)},
				Responder: &Responder{Claims: []TouchClaim{{}}, OnMove: func(e ResponderEvent) {
					moves = append(moves, e)
				}}},
			View{TestID: "other", Style: Style{Width: Pt(50), Height: Pt(50)}, OnTouch: func(TouchEvent) {}},
		}}
	})
	h.commit()
	owner, other := h.find("owner").id, h.find("other").id
	report := func(phase GesturePhase, touches []Touch, targets map[int]NodeID, points []Point) {
		h.tree.Dispatch(driver.Event{Kind: driver.EventTouch, Phase: phase, Touches: touches,
			TouchGeometry: &driver.TouchGeometry{Targets: targets, Positions: map[NodeID][]Point{owner: points}}})
	}
	a := Touch{ID: 7, PagePosition: Point{X: 200, Y: 200}, Changed: true}
	b := Touch{ID: 9, PagePosition: Point{X: 250, Y: 260}, Changed: true}
	report(GestureBegan, []Touch{a}, map[int]NodeID{7: owner}, []Point{{X: 10, Y: 20}})
	a.Changed = false
	report(GestureBegan, []Touch{a, b}, map[int]NodeID{9: other}, []Point{{X: 10, Y: 20}, {X: 30, Y: 40}})
	a.Changed, b.Changed = true, false
	report(GestureEnded, []Touch{a, b}, nil, []Point{{X: 10, Y: 20}, {X: 30, Y: 40}})
	b.Changed = true
	report(GestureChanged, []Touch{b}, nil, []Point{{X: 35, Y: 45}})
	if len(moves) != 1 || moves[0].Position != (Point{X: 35, Y: 45}) || moves[0].PagePosition != b.PagePosition {
		t.Fatalf("remaining finger at its original responder = %+v", moves)
	}
	report(GestureEnded, []Touch{b}, nil, []Point{{X: 35, Y: 45}})
}
