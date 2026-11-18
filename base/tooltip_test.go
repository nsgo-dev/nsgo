package base_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestTooltipDescriptionsReachTheControlsBeforeFocus(t *testing.T) {
	var text, label, explicit *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		text = core.NewSignal(cx, "Saves the current document")
		label = core.NewSignal(cx, "Save")
		explicit = core.NewSignal(cx, "The app's description")
		return core.View{Style: core.Style{Padding: core.All(40)}, Children: []core.Node{
			base.Tooltip{TestID: "wrapper", Trigger: base.Button{TestID: "save", LabelFunc: label.Get, OnPress: func(core.PressEvent) {},
				Style: core.Style{Height: core.Pt(30)}},
				Children: []core.Node{core.View{Children: []core.Node{
					core.Text{ContentFunc: text.Get, TestID: "tip"},
					core.View{ImportantForAccessibility: core.ImportantNoHideDescendants, Children: []core.Node{core.Text{Content: "decoration"}}},
				}}}},
			core.View{TestID: "explicit", Label: "Other", AccessibilityDescriptionFunc: explicit.Get,
				Style: core.Style{Height: core.Pt(30)}, OnPress: func(core.PressEvent) {}, Tooltip: core.Text{Content: "A fallback description"}},
		}}
	}, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, OS: core.OSMacOS}))
	check := func(id, want string) {
		t.Helper()
		if got := app.Find("id:" + id).Props.Description; got != want {
			t.Fatalf("%s description = %q, want %q", id, got, want)
		}
	}
	check("save", "Saves the current document")
	check("wrapper", "")
	check("explicit", "The app's description")
	if app.Exists("id:tip") {
		t.Fatal("the tooltip is visible before the trigger has focus")
	}
	app.Key("tab")
	bubble := app.Find("id:tip")
	for v := bubble; ; v = v.Parent {
		if v == nil {
			t.Fatal("the tooltip has no overlay")
		}
		if !v.Props.ElementsHidden || v.Props.Role == core.RoleAlert {
			t.Fatalf("the tooltip exposes separate screen-reader content: %+v", v.Props)
		}
		if v.Props.PointerEvents == core.PointerEventsNone {
			break
		}
	}
	app.Key("escape")
	app.Post(func() { text.Set("Save") })
	app.Settle()
	check("save", "")
	app.Post(func() { text.Set("Save\n   document") })
	app.Settle()
	check("save", "Save document")
	app.Post(func() { label.Set("Save document") })
	app.Settle()
	check("save", "")
	app.Post(func() { explicit.Set("") })
	app.Settle()
	check("explicit", "A fallback description")
}

func TestTooltipDescriptionsFollowRichContentAndTriggerReplacement(t *testing.T) {
	var show, first *core.Signal[bool]
	var caption *core.Signal[string]
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		show, first = core.NewSignal(cx, true), core.NewSignal(cx, true)
		caption = core.NewSignal(cx, "Alpha")
		return core.View{Children: []core.Node{
			base.Tooltip{Trigger: core.Dynamic(func(*core.Scope) core.Node {
				if first.Get() {
					return base.Button{TestID: "first", Label: "First", Style: core.Style{Height: core.Pt(30)}}
				}
				return core.View{Children: []core.Node{core.Input{TestID: "second"}}}
			}), Children: []core.Node{
				core.Text{Spans: []core.Span{{Content: "Read "}, {ContentFunc: caption.Get}}},
				core.Show{When: show.Get, Then: func(*core.Scope) core.Node { return core.Text{Content: "details"} }},
			}},
		}}
	})
	if got := app.Find("id:first").Props.Description; got != "Read Alpha details" {
		t.Fatalf("rich description = %q", got)
	}
	app.Post(func() {
		first.Set(false)
		show.Set(false)
		caption.Set("Beta")
	})
	app.Settle()
	if got := app.Find("id:second").Props.Description; got != "Read Beta" {
		t.Fatalf("the replacement input's description = %q", got)
	}
}

func TestPlainTooltipDoesNotBecomeTheUnlabelledTriggersName(t *testing.T) {
	app := startDesktop(t, func(*core.Scope) core.Node {
		return core.View{TestID: "control", Role: core.RoleButton, Accessible: true,
			Style:    core.Style{Width: core.Pt(100), Height: core.Pt(30)},
			Children: []core.Node{core.Text{Content: "Save"}}, Tooltip: core.Text{Content: "Writes the document"}}
	})
	v := app.Find("id:control")
	if v.Props.Label != "Save" || v.Props.Description != "Writes the document" {
		t.Fatalf("trigger props = %+v", v.Props)
	}
	app.Hover("id:control")
	app.Advance(600 * time.Millisecond)
	v = app.Find("id:control")
	if v.Props.Label != "Save" {
		t.Fatalf("the visible tooltip changed the trigger's name to %q", v.Props.Label)
	}
}

// A closed tooltip's bubble stays mounted, hidden, for its description, so it waits in its starting part: showing it again changes its display and nothing its transition would animate from the shown or ending style.
func TestTooltipWaitsInItsStartingStyleWhileHidden(t *testing.T) {
	var open *core.Signal[bool]
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		open = core.NewSignal(cx, false)
		return base.Tooltip{Open: open.Get, Trigger: base.Button{Label: "Save", Style: core.Style{Height: core.Pt(30)}},
			PopupStyle:    core.Style{Transition: core.Transition{Duration: 200 * time.Millisecond, Properties: core.TransitionOpacity}},
			StartingClass: "scale-90", StartingStyle: core.Style{Opacity: core.Alpha(0)}, EndingStyle: core.Style{Opacity: core.Alpha(0.5)},
			Children: []core.Node{core.Text{Content: "Writes the document", TestID: "tip"}}}
	})
	bubble := func() *nsgotest.View {
		t.Helper()
		for v := app.FindMounted("id:tip"); v != nil; v = v.Parent {
			if v.Props.PointerEvents == core.PointerEventsNone {
				return v
			}
		}
		t.Fatal("the tooltip has no overlay")
		return nil
	}
	waiting := func(when string) {
		t.Helper()
		b := bubble()
		if !b.Style.Hidden || b.Style.Opacity != 0 || b.Style.Transform == driver.DefaultViewStyle.Transform {
			t.Fatalf("%s: the hidden bubble does not wait in its starting part: hidden %v, opacity %v, transform %v", when, b.Style.Hidden, b.Style.Opacity, b.Style.Transform)
		}
	}
	waiting("before it first shows")
	for round := range 2 {
		app.Post(func() { open.Set(true) })
		app.Settle()
		if b := bubble(); b.Style.Hidden || b.Style.Opacity != 0 {
			t.Fatalf("round %d: the bubble shows at opacity %v, hidden %v", round, b.Style.Opacity, b.Style.Hidden)
		}
		app.Advance(60 * time.Millisecond)
		if got := bubble().Style.Opacity; got != 1 {
			t.Fatalf("round %d: past its first frame the bubble's opacity is %v", round, got)
		}
		app.Post(func() { open.Set(false) })
		app.Settle()
		if b := bubble(); b.Style.Hidden || b.Style.Opacity != 0.5 {
			t.Fatalf("round %d: closing, the bubble's opacity is %v, hidden %v", round, b.Style.Opacity, b.Style.Hidden)
		}
		app.Advance(time.Second)
		waiting("after it closed")
	}
}

// A disabled tooltip never shows, so it gives its trigger no description either.
func TestDisabledTooltipDescribesNothing(t *testing.T) {
	app := startDesktop(t, func(*core.Scope) core.Node {
		return base.Tooltip{Disabled: true, Trigger: base.Button{Label: "Save", TestID: "save", Style: core.Style{Height: core.Pt(30)}, OnPress: func(core.PressEvent) {}},
			Children: []core.Node{core.Text{Content: "Writes the document", TestID: "tip"}}}
	})
	if got := app.Find("id:save").Props.Description; got != "" {
		t.Fatalf("a disabled tooltip described its button as %q", got)
	}
	app.Key("tab")
	if app.Exists("id:tip") {
		t.Fatal("a disabled tooltip showed for the keyboard's focus")
	}
}
