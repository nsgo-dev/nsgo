package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestLabelForNamesAndActivatesControl(t *testing.T) {
	field, toggle := new(Ref), new(Ref)
	var name *Signal[string]
	var events []PressEvent
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		name = NewSignal(cx, "Email")
		return View{Children: []Node{
			Text{TestID: "label", ContentFunc: name.Get, LabelFor: field},
			Input{TestID: "field", Ref: field},
			View{TestID: "switch-label", LabelFor: toggle, Children: []Node{Text{Content: "Updates"}}},
			View{TestID: "switch", Ref: toggle, Role: RoleSwitch, OnPress: func(e PressEvent) { events = append(events, e) }},
		}}
	})
	h.commit()
	label, input := h.find("label"), h.find("field")
	if p := input.sent.props; p.Label != "Email" || p.LabelledBy == nil || !slices.Equal(*p.LabelledBy, []NodeID{label.id}) {
		t.Fatalf("field = %+v", p)
	}
	if p := label.sent.props; p.Role != RoleText || !p.NoFocus || !p.LabelPress || p.LabelFor != input.id || !label.sent.events.Has(driver.EventPress) {
		t.Fatalf("label = %+v, events %v", p, label.sent.events)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: label.id, Index: -1})
	if c := command(h.commit(), driver.ItemFocus); c == nil || c.ID != input.id {
		t.Fatalf("pressing label did not focus the input: %+v", c)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventInputModality, Count: driver.ModalityKeyboard})
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: h.find("switch-label").id, Index: -1, Point: Point{X: 12, Y: 5}, Modifiers: Modifiers{Shift: true}})
	if c := command(h.commit(), driver.ItemFocus); c == nil || c.ID != toggle.n.id || len(events) != 1 || !events[0].Keyboard || !events[0].Modifiers.Shift || events[0].Position != (Point{}) {
		t.Fatalf("label activation = %+v, focus %+v", events, c)
	}
	name.Set("E-mail address")
	h.commit()
	if got := input.sent.props.Label; got != "E-mail address" {
		t.Fatalf("updated name = %q", got)
	}
}

func TestLabelledByPrecedenceOrderAndVisibleText(t *testing.T) {
	a, b, field := new(Ref), new(Ref), new(Ref)
	var content *Signal[string]
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		content = NewSignal(cx, "Name")
		return View{Children: []Node{
			Text{Ref: a, SpansFunc: func() []Span {
				return []Span{{Content: content.Get()}, {Content: " secret", Style: Style{Visibility: VisibilityHidden}}}
			}},
			View{Ref: b, Children: []Node{Text{Content: "Required"}}},
			Text{Content: "Implicit", LabelFor: field},
			Input{Ref: field, TestID: "field", Label: "Fallback", LabelledBy: []*Ref{b, nil, b, a}},
		}}
	})
	h.commit()
	p := field.n.sent.props
	if p.Label != "Required Name" || p.LabelledBy == nil || !slices.Equal(*p.LabelledBy, []NodeID{b.n.id, a.n.id}) {
		t.Fatalf("named field = %+v", p)
	}
	content.Set("Nom")
	h.commit()
	if p := field.n.sent.props; p.Label != "Required Nom" {
		t.Fatalf("renamed field = %+v", p)
	}
}

func TestLabelStatesFollowControlAndRestoreOwnProps(t *testing.T) {
	ref := new(Ref)
	var off, bad, show *Signal[bool]
	presses := 0
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		off, bad, show = NewSignal(cx, false), NewSignal(cx, false), NewSignal(cx, true)
		return View{Children: []Node{
			View{TestID: "label", LabelFor: ref, Class: "disabled:opacity-50 aria-invalid:text-red-500", Children: []Node{Text{Content: "Choice"}}},
			Show{When: show.Get, Then: func(*Scope) Node {
				return View{TestID: "control", Ref: ref, Role: RoleCheckbox, Disabled: off.Get, Invalid: bad.Get, OnPress: func(PressEvent) { presses++ }}
			}},
		}}
	})
	h.commit()
	label := h.find("label")
	off.Set(true)
	bad.Set(true)
	h.commit()
	if p := label.sent.props; !p.Disabled || !p.Invalid || label.style.Opacity != Alpha(.5) || label.style.Color == 0 {
		t.Fatalf("projected label = %+v, style %+v", p, label.style)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: label.id, Index: -1})
	if command(h.commit(), driver.ItemFocus) != nil || presses != 0 {
		t.Fatal("disabled label activated control")
	}
	off.Set(false)
	bad.Set(false)
	h.commit()
	if p := label.sent.props; p.Disabled || p.Invalid || label.style.Opacity == Alpha(.5) {
		t.Fatalf("states stuck after reset: %+v", p)
	}
	off.Set(true)
	bad.Set(true)
	h.commit()
	show.Set(false)
	h.commit()
	if p := label.sent.props; p.Disabled || p.Invalid || p.LabelFor != 0 || p.LabelPress || label.sent.events.Has(driver.EventPress) {
		t.Fatalf("states or activation stuck after removal: %+v", p)
	}
	show.Set(true)
	h.commit()
	if p := label.sent.props; !p.Disabled || !p.Invalid || p.LabelFor != ref.n.id {
		t.Fatalf("states lost after remount: %+v", p)
	}
}

func TestLabelAssociationChangesAndDetachedRefs(t *testing.T) {
	a, b := new(Ref), new(Ref)
	var root, label, target *Element
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		root = NewElement(cx, View{})
		label = NewElement(cx, Text{Content: "First", LabelFor: a})
		target = NewElement(cx, Input{Ref: a, Label: "Own"})
		root.Insert(label, nil)
		root.Insert(target, nil)
		root.Insert(NewElement(cx, Input{Ref: b}), nil)
		return root
	})
	h.commit()
	if a.n.sent.props.Label != "Own" || a.n.sent.props.LabelledBy != nil {
		t.Fatal("implicit label replaced explicit name")
	}
	label.Set(Text{Content: "Second", LabelFor: b})
	h.commit()
	if b.n.sent.props.Label != "Second" || a.n.sent.props.Label != "Own" {
		t.Fatal("changing target did not clear old association")
	}
	root.Remove(label)
	h.commit()
	if p := b.n.sent.props; p.Label != "" || p.LabelledBy != nil {
		t.Fatalf("detached label still names target: %+v", p)
	}
	root.Insert(label, nil)
	h.commit()
	if b.n.sent.props.Label != "Second" {
		t.Fatal("reattached label did not name target")
	}
	label.Set(Text{Content: "Third", LabelFor: a})
	target.Set(Input{Ref: a})
	h.commit()
	if a.n.sent.props.Label != "Third" || b.n.sent.props.Label != "" {
		t.Fatal("updating element props left an old name")
	}
	root.Remove(target)
	h.commit()
	if label.n.sent.props.LabelFor != 0 {
		t.Fatal("detached target remained activatable")
	}
	root.Insert(target, nil)
	h.commit()
	if a.n.sent.props.Label != "Third" {
		t.Fatal("reattached target did not recover name")
	}
}

func TestImplicitLabelsFollowTreeOrder(t *testing.T) {
	field := new(Ref)
	var root, a, b *Element
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		root = NewElement(cx, View{})
		a = NewElement(cx, Text{Content: "Alpha", LabelFor: field})
		b = NewElement(cx, Text{Content: "Beta", LabelFor: field})
		root.Insert(a, nil)
		root.Insert(b, nil)
		root.Insert(NewElement(cx, Input{Ref: field}), nil)
		return root
	})
	h.commit()
	if field.n.sent.props.Label != "Alpha Beta" {
		t.Fatal(field.n.sent.props.Label)
	}
	root.Insert(b, a)
	h.commit()
	if field.n.sent.props.Label != "Beta Alpha" {
		t.Fatal(field.n.sent.props.Label)
	}
}

func TestLabelCyclesAndCrossTreeRefsAreIgnored(t *testing.T) {
	a, b, foreign := new(Ref), new(Ref), new(Ref)
	other := newHost(t, Size{Width: 100, Height: 100}, func(*Scope) Node { return Text{Ref: foreign, Content: "Other window"} })
	other.commit()
	h := newHost(t, Size{Width: 300, Height: 300}, func(*Scope) Node {
		return View{Children: []Node{
			Text{Ref: a, Content: "Alpha", LabelledBy: []*Ref{b}},
			Text{Ref: b, Content: "Beta", LabelledBy: []*Ref{a}},
			Input{TestID: "field", Label: "Fallback", LabelledBy: []*Ref{foreign, a, a}},
			Text{TestID: "foreign-label", Content: "Leak", LabelFor: foreign},
		}}
	})
	h.commit()
	if a.n.sent.props.LabelledBy != nil || b.n.sent.props.LabelledBy != nil {
		t.Fatal("cyclic native relationships survived")
	}
	if p := h.find("field").sent.props; p.Label != "Alpha" || p.LabelledBy == nil || len(*p.LabelledBy) != 1 {
		t.Fatalf("field = %+v", p)
	}
	if h.find("foreign-label").sent.props.LabelFor != 0 {
		t.Fatal("cross-tree activation survived")
	}
	if len(h.commit().Items) != 0 {
		t.Fatal("cyclic relationships never settled")
	}
}

func TestLabelCannotCrossConfinementOrActivateBelowModal(t *testing.T) {
	outside := new(Ref)
	presses := 0
	h := newHost(t, Size{Width: 300, Height: 300}, func(*Scope) Node {
		return View{Children: []Node{
			View{Ref: outside, OnPress: func(PressEvent) { presses++ }},
			Func(func(cx *Scope) Node {
				cx.Confine()
				return Text{TestID: "confined", Content: "Label", LabelFor: outside, LabelledBy: []*Ref{outside}}
			}),
			Modal{Children: []Node{Text{TestID: "modal-label", Content: "Outside", LabelFor: outside}}},
		}}
	})
	h.commit()
	if p := h.find("confined").sent.props; p.LabelFor != 0 || p.LabelledBy != nil {
		t.Fatalf("confinement bypass: %+v", p)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: h.find("modal-label").id, Index: -1})
	if command(h.commit(), driver.ItemFocus) != nil || presses != 0 {
		t.Fatal("modal label activated a covered control")
	}
}
