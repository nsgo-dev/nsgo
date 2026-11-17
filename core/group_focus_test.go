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
