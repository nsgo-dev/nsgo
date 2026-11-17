package base_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
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
