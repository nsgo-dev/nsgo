package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestTooltipDescriptionOmitsHiddenContent(t *testing.T) {
	h := newHost(t, Size{Width: 300, Height: 300}, func(*Scope) Node {
		return View{TestID: "trigger", Label: "Save", Role: RoleButton, Tooltip: Group{
			Text{Content: "Writes", LiveRegion: LiveRegionAssertive, Role: RoleAlert, TestID: "alert"},
			View{Style: Style{Visibility: VisibilityHidden}, Children: []Node{Text{Content: "secret"}}},
			View{Style: Style{Display: DisplayNone}, Children: []Node{Text{Content: "absent"}}},
			View{ImportantForAccessibility: ImportantNoHideDescendants, Label: "decoration"},
			Text{Spans: []Span{{Content: "the document"}, {Content: " hidden", Style: Style{Visibility: VisibilityHidden}}}},
		}}
	})
	h.commit()
	if got := h.find("trigger").sent.props.Description; got != "Writes the document" {
		t.Fatalf("tooltip description = %q", got)
	}
	if p := h.find("alert").sent.props; !p.ElementsHidden || p.Accessible != driver.AccessibleNo || p.LiveRegion != LiveRegionNone || p.Role != RoleNone {
		t.Fatalf("tooltip announcement props = %+v", p)
	}
}

func TestTooltipDescriptionsFollowMovedControls(t *testing.T) {
	var root, trigger, control *Element
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		root = NewElement(cx, View{})
		trigger = NewElement(cx, View{Tooltip: Text{Content: "Describes the field"}})
		control = NewElement(cx, Input{TestID: "field", Label: "Name"})
		root.Insert(trigger, nil)
		trigger.Insert(control, nil)
		return root
	})
	h.commit()
	if got := control.n.sent.props.Description; got != "Describes the field" {
		t.Fatalf("wrapped control description = %q", got)
	}
	root.Insert(control, nil)
	h.commit()
	if got := control.n.sent.props.Description; got != "" {
		t.Fatalf("moved control retained description = %q", got)
	}
	trigger.Insert(control, nil)
	h.commit()
	if got := control.n.sent.props.Description; got != "Describes the field" {
		t.Fatalf("returned control description = %q", got)
	}
	root.Remove(trigger)
	h.commit()
	if got := control.n.tooltipDescription; got != "" {
		t.Fatalf("detached control retained description = %q", got)
	}
	root.Insert(trigger, nil)
	h.commit()
	if got := control.n.sent.props.Description; got != "Describes the field" {
		t.Fatalf("reattached control description = %q", got)
	}
}

func TestTooltipDescriptionDeduplicatesResolvedLabels(t *testing.T) {
	ref := new(Ref)
	var label *Signal[string]
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		label = NewSignal(cx, "Save")
		return View{Children: []Node{
			Text{Ref: ref, ContentFunc: label.Get},
			View{TestID: "trigger", LabelledBy: []*Ref{ref}, Tooltip: Text{Content: "Save"}},
		}}
	})
	h.commit()
	trigger := h.find("trigger")
	if got := trigger.sent.props.Description; got != "" {
		t.Fatalf("duplicate description = %q", got)
	}
	label.Set("Write")
	h.commit()
	if got := trigger.sent.props.Description; got != "Save" {
		t.Fatalf("description after renaming = %q", got)
	}
}

func TestMovingContentIntoAndOutOfTooltipUpdatesItsAccessibility(t *testing.T) {
	var root, tip, content, text *Element
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		root, tip = NewElement(cx, View{}), NewElement(cx, nil)
		content = NewElement(cx, View{})
		text = NewElement(cx, Text{Content: "Status", Role: RoleStatus})
		content.Insert(text, nil)
		root.Insert(NewElement(cx, View{Tooltip: tip}), nil)
		root.Insert(content, nil)
		return root
	})
	h.commit()
	tip.Insert(content, nil)
	h.commit()
	if p := text.n.sent.props; !p.ElementsHidden || p.LiveRegion != LiveRegionNone || p.Role != RoleNone {
		t.Fatalf("moved tooltip text is exposed: %+v", p)
	}
	root.Insert(content, nil)
	h.commit()
	if p := text.n.sent.props; p.ElementsHidden || p.Role != RoleStatus || p.LiveRegion != LiveRegionPolite {
		t.Fatalf("text kept tooltip accessibility after leaving: %+v", p)
	}
}

// A closed tooltip's hidden overlay stays mounted for its description, so it must not count as an anchored overlay that a scroll moves: otherwise every scroll lays out the whole tree and places every hidden bubble.
func TestHiddenTooltipDoesNotFollowScrolls(t *testing.T) {
	h := newHost(t, Size{Width: 300, Height: 300}, func(*Scope) Node {
		return Scroll{TestID: "scroll", Style: Style{FlexGrow: 1}, Children: []Node{
			View{TestID: "trigger", Label: "Delete", Role: RoleButton, Style: Style{Width: Pt(20), Height: Pt(20)}, Tooltip: Text{Content: "Deletes the row"}},
			View{Style: Style{Height: Pt(1000)}},
		}}
	})
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: h.find("scroll").id, Point: Point{Y: 40}})
	if h.tree.view.layoutDirty {
		t.Fatal("a scroll lays the tree out again for a hidden tooltip")
	}
	h.commit()
	h.tree.within(h.tree.root, func() { h.find("trigger").tip.focus(true) })
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: h.find("scroll").id, Point: Point{Y: 80}})
	if !h.tree.view.layoutDirty {
		t.Fatal("a shown tooltip does not follow a scroll")
	}
}
