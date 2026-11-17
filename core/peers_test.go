package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestPeerValueStatesAndNames(t *testing.T) {
	var on *Signal[bool]
	variants := []string{"checked", "indeterminate", "selected", "disabled", "aria-checked", "aria-selected", "aria-expanded", "aria-busy", "aria-pressed", "aria-invalid", "data-invalid"}
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		on = NewSignal(cx, false)
		children := []Node{
			View{TestID: "before", Class: "peer-checked:bg-[#ff0000]"},
			View{Class: "peer peer/choice", Checked: on.Get, Indeterminate: on.Get, Selected: on.Get,
				Disabled: on.Get, Expanded: on.Get, Busy: on.Get, Pressed: on.Get, Invalid: on.Get,
				PartStates: func() PartStates {
					if on.Get() {
						return PartStates{Data: []string{"invalid"}}
					}
					return PartStates{}
				}, Children: []Node{View{TestID: "inside", Class: "peer-checked:bg-[#ff0000]"}}},
			View{},                          // The marker need not be adjacent to its followers.
			View{Class: "peer peer/choice"}, // A later inactive peer does not hide an earlier match.
			View{TestID: "named", Class: "peer-checked/choice:bg-[#ff0000]"},
			View{TestID: "missing", Class: "peer-checked/other:bg-[#ff0000]"},
			View{TestID: "enabled", Class: "peer-enabled:bg-[#ff0000]"},
			View{Children: []Node{View{TestID: "nested", Class: "peer-checked:bg-[#ff0000]"}}},
			View{Children: []Node{
				View{Class: "peer/named-only", Checked: on.Get},
				View{TestID: "unnamed", Class: "peer-checked:bg-[#ff0000]"},
			}},
		}
		for _, variant := range variants {
			children = append(children, View{TestID: variant, Class: "peer-" + variant + ":bg-[#ff0000]"})
		}
		return View{Children: children}
	})
	for _, value := range []bool{false, true, false} {
		on.Set(value)
		h.commit()
		color := Color(0)
		if value {
			color = RGB(0xff0000)
		}
		for _, id := range append(slices.Clone(variants), "named") {
			want(t, id, h.find(id).style.Background, color)
		}
		// Any enabled peer matches, including the second, unchecked peer.
		want(t, "enabled", h.find("enabled").style.Background, RGB(0xff0000))
		for _, id := range []string{"before", "inside", "nested", "missing", "unnamed"} {
			want(t, id, h.find(id).style.Background, Color(0))
		}
	}
}

func TestPeerInteractionsAndMixedVariants(t *testing.T) {
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		return View{TestID: "root", Class: "group", Children: []Node{
			View{TestID: "source", Class: "peer", OnPress: press},
			View{TestID: "hover", Class: "peer-hover:bg-[#ff0000]"},
			View{TestID: "active", Class: "peer-active:bg-[#ff0000]"},
			View{TestID: "focus", Class: "peer-focus:bg-[#ff0000]"},
			View{TestID: "visible", Class: "peer-focus-visible:bg-[#ff0000]"},
			View{TestID: "mixed", Class: "peer-hover:hover:bg-[#ff0000]", OnPress: press},
			View{TestID: "group", Class: "group-hover:peer-hover:bg-[#ff0000]"},
			View{TestID: "wrapper", Class: "peer/field", Children: []Node{Input{TestID: "input"}}},
			View{TestID: "within", Class: "peer-focus-within/field:bg-[#ff0000]"},
			View{TestID: "bare", Class: "peer"},
		}}
	})
	h.commit()
	for _, kind := range []driver.EventKind{driver.EventMouseEnter, driver.EventMouseLeave, driver.EventPressDown, driver.EventPressUp} {
		if !h.find("bare").events().Has(kind) {
			t.Errorf("bare peer does not request %v", kind)
		}
	}
	send := func(id string, kind driver.EventKind) {
		h.tree.Dispatch(driver.Event{Kind: kind, ID: h.find(id).id})
		h.commit()
	}
	send("source", driver.EventMouseEnter)
	want(t, "peer hover", h.find("hover").style.Background, RGB(0xff0000))
	want(t, "own hover still required", h.find("mixed").style.Background, Color(0))
	send("mixed", driver.EventMouseEnter)
	want(t, "both hovered", h.find("mixed").style.Background, RGB(0xff0000))
	// Platforms report entering the containing group as well.
	send("root", driver.EventMouseEnter)
	want(t, "group and peer", h.find("group").style.Background, RGB(0xff0000))
	send("source", driver.EventPressDown)
	want(t, "peer active", h.find("active").style.Background, RGB(0xff0000))
	send("source", driver.EventPressUp)
	want(t, "peer inactive", h.find("active").style.Background, Color(0))
	send("source", driver.EventFocus)
	want(t, "peer focus", h.find("focus").style.Background, RGB(0xff0000))
	want(t, "peer visible focus", h.find("visible").style.Background, RGB(0xff0000))
	send("source", driver.EventBlur)
	want(t, "peer blur", h.find("focus").style.Background, Color(0))
	h.tree.Dispatch(driver.Event{Kind: driver.EventInputModality, Count: driver.ModalityPointer})
	send("source", driver.EventFocus)
	want(t, "pointer focus", h.find("visible").style.Background, Color(0))
	h.tree.Dispatch(driver.Event{Kind: driver.EventInputModality, Count: driver.ModalityKeyboard})
	h.commit()
	want(t, "keyboard focus", h.find("visible").style.Background, RGB(0xff0000))
	send("source", driver.EventBlur)
	send("source", driver.EventMouseLeave)
	want(t, "peer left", h.find("hover").style.Background, Color(0))
	want(t, "mixed peer left", h.find("mixed").style.Background, Color(0))
	send("input", driver.EventFocus)
	want(t, "peer focus within", h.find("within").style.Background, RGB(0xff0000))
	send("input", driver.EventBlur)
	want(t, "peer focus left", h.find("within").style.Background, Color(0))
}

func TestPeersFollowElementMovesAndMarkerChanges(t *testing.T) {
	var root, peer, label, elsewhere *Element
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		root = NewElement(cx, View{})
		peer = NewElement(cx, View{TestID: "peer", Class: "peer", Checked: func() bool { return true }})
		label = NewElement(cx, View{TestID: "label", Class: "peer-checked:bg-[#ff0000] peer-aria-invalid:border-[3px]"})
		elsewhere = NewElement(cx, View{})
		root.Insert(peer, nil)
		root.Insert(label, nil)
		return View{Children: []Node{root, elsewhere}}
	})
	check := func(color Color) {
		h.commit()
		want(t, "label background", h.find("label").style.Background, color)
	}
	check(RGB(0xff0000))
	root.Insert(peer, nil)
	check(0)
	root.Insert(peer, label)
	check(RGB(0xff0000))
	peer.Set(View{Class: "peer", Invalid: func() bool { return true }})
	check(0)
	want(t, "imperative invalid", h.find("label").style.BorderWidth, Points(3))
	peer.Set(View{Checked: func() bool { return true }})
	check(0)
	want(t, "invalid cleared", h.find("label").style.BorderWidth, Points(0))
	peer.Set(View{Class: "peer", Checked: func() bool { return true }})
	check(RGB(0xff0000))
	elsewhere.Insert(peer, nil)
	check(0)
	root.Insert(peer, label)
	check(RGB(0xff0000))
	root.Remove(peer)
	check(0)
}

func TestPeerInputDisabledAndCombinedNames(t *testing.T) {
	var checked, disabled *Signal[bool]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		checked, disabled = NewSignal(cx, false), NewSignal(cx, false)
		return View{Children: []Node{
			View{Class: "peer/a", Checked: checked.Get},
			Input{Class: "peer/b", Disabled: disabled.Get},
			View{TestID: "combined", Class: "peer-checked/a:peer-disabled/b:bg-[#ff0000]"},
			View{TestID: "enabled", Class: "peer-enabled/b:bg-[#ff0000]"},
			View{TestID: "disabled", Class: "peer-disabled/b:bg-[#ff0000]"},
		}}
	})
	for _, state := range [][2]bool{{false, false}, {true, false}, {true, true}, {false, true}, {false, false}} {
		checked.Set(state[0])
		disabled.Set(state[1])
		h.commit()
		for id, active := range map[string]bool{"combined": state[0] && state[1], "enabled": !state[1], "disabled": state[1]} {
			color := Color(0)
			if active {
				color = RGB(0xff0000)
			}
			want(t, id, h.find(id).style.Background, color)
		}
	}
}

func TestPeerRegionsClassesAndSpacing(t *testing.T) {
	var shown, marked, checked *Signal[bool]
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		shown, marked, checked = NewSignal(cx, true), NewSignal(cx, true), NewSignal(cx, true)
		return View{Children: []Node{
			Show{When: shown.Get, Then: func(*Scope) Node {
				return View{ClassFunc: func() string {
					if marked.Get() {
						return "peer"
					}
					return ""
				}, PartStates: func() PartStates { return PartStates{Checked: checked.Get()} }}
			}},
			View{TestID: "following", Class: "peer-checked:bg-[#ff0000] peer-checked:space-y-4", Children: []Node{
				View{TestID: "first"}, View{},
			}},
		}}
	})
	for _, state := range [][3]bool{{true, true, true}, {true, true, false}, {true, false, true}, {false, true, true}, {true, true, true}} {
		shown.Set(state[0])
		marked.Set(state[1])
		checked.Set(state[2])
		h.commit()
		color, margin := Color(0), Points(0)
		if state[0] && state[1] && state[2] {
			color, margin = RGB(0xff0000), 16
		}
		want(t, "region peer", h.find("following").style.Background, color)
		want(t, "conditional child spacing", h.find("first").style.Margin.Bottom, margin)
	}
}

func TestPeerParser(t *testing.T) {
	if unknown := UnknownClasses("peer peer/field peer-hover:p-1 peer-enabled:p-2 peer-checked/field:p-3 peer-data-invalid:p-4 peer-aria-invalid:p-5"); len(unknown) != 0 {
		t.Fatalf("unknown peer classes: %v", unknown)
	}
	for _, class := range []string{"peer/", "peer-hover/:p-1", "peer-bogus:p-1", "peer-data-:p-1", "peer-data-[state=open]:p-1"} {
		if unknown := UnknownClasses(class); !slices.Equal(unknown, []string{class}) {
			t.Errorf("%s accepted malformed or unsupported peer selector", class)
		}
	}
}
