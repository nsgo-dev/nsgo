package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestStyleMarkersDoNotCreateFocusStops(t *testing.T) {
	for _, v := range []View{
		{Class: "group"},
		{Class: "group/card"},
		{Style: Style{Group: "card"}},
		{Class: "active:opacity-50"},
		{Style: Style{Active: &Style{Opacity: Alpha(.5)}}},
	} {
		name := v.Class
		if name == "" {
			name = "style"
		}
		t.Run(name, func(t *testing.T) {
			v.TestID = "marker"
			h := newHost(t, Size{Width: 100, Height: 100}, func(*Scope) Node { return v })
			n := h.find("marker")
			m := n.events()
			if m.Pressable() || n.props.FocusStop(m) {
				t.Fatalf("style-only view takes presses or keyboard focus: %b", m)
			}
			if n.style.Group != "" && (!m.Has(driver.EventMouseEnter) || !m.Has(driver.EventMouseLeave)) {
				t.Fatal("group no longer reports hover")
			}
		})
	}
}

// A responder's Active style follows its touches, not press events, so it needs no press events and stays out of the keyboard stops, as a scrollbar's thumb does.
func TestAResponderShowsActiveWithoutBecomingAFocusStop(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(*Scope) Node {
		return View{TestID: "thumb", Style: Style{Height: Pt(50), Active: &Style{Opacity: Alpha(.5)}}, Responder: &Responder{Claims: []TouchClaim{{}}}}
	})
	h.commit()
	n := h.find("thumb")
	if m := n.events(); m.Pressable() || n.props.FocusStop(m) {
		t.Fatalf("a responder's Active style made it take presses or keyboard focus: %b", m)
	}
	h.tree.Dispatch(finger(GestureBegan, Point{X: 20, Y: 20}, 0))
	if n.style.Opacity.Value() != .5 {
		t.Errorf("opacity = %v while the responder holds the touch", n.style.Opacity.Value())
	}
	h.tree.Dispatch(finger(GestureEnded, Point{X: 20, Y: 20}, 16))
	if n.style.Opacity.Value() != 1 {
		t.Errorf("opacity = %v after the touch ended", n.style.Opacity.Value())
	}
}
