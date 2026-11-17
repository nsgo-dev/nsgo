package press_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// The same test presses real label views on native targets and verifies the input's focus, control activation and dynamic state restoration.
func TestVisibleLabelsFocusAndActivateTheirControls(t *testing.T) {
	var scope *core.Scope
	var name *core.Signal[string]
	var off, bad, checked *core.Signal[bool]
	field, toggle := new(core.Ref), new(core.Ref)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		name = core.NewSignal(cx, "Email")
		off, bad, checked = core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return core.View{Style: core.Style{Padding: core.All(20), Gap: 8}, Children: []core.Node{
			core.Text{TestID: "field-label", LabelFor: field, ContentFunc: name.Get, Style: core.Style{Height: core.Pt(32)}},
			base.Input{TestID: "field", Ref: field, Style: core.Style{Height: core.Pt(32)}},
			core.View{TestID: "check-label", LabelFor: toggle, Class: "disabled:opacity-50 aria-invalid:text-red-500", Style: core.Style{Height: core.Pt(32)}, Children: []core.Node{core.Text{Content: "Updates"}}},
			base.Checkbox{TestID: "check", Ref: toggle, Checked: checked.Get, Disabled: off.Get, Invalid: bad.Get, OnChange: checked.Set, Style: core.Style{Height: core.Pt(32)}},
		}}
	}, nsgotest.Size(320, 300))
	app.Press("id:field-label")
	if !app.Find("id:field").Focused {
		t.Fatal("pressing visible label did not focus input")
	}
	if p := app.Find("id:field").Props; p.Label != "Email" {
		t.Fatalf("input name = %+v", p)
	}
	app.Press("id:check-label")
	if !app.Find("id:check").Props.Checked {
		t.Fatal("pressing visible label did not check checkbox")
	}
	scope.Post(func() { name.Set("Address"); off.Set(true); bad.Set(true) })
	app.Advance(0)
	if p := app.Find("id:field").Props; p.Label != "Address" {
		t.Fatalf("updated name = %+v", p)
	}
	if p := app.Find("id:check-label").Props; !p.Disabled || !p.Invalid {
		t.Fatalf("label states = %+v", p)
	}
	scope.Post(func() { off.Set(false); bad.Set(false) })
	app.Advance(0)
	app.Press("id:check-label")
	if app.Find("id:check").Props.Checked {
		t.Fatal("re-enabled label did not uncheck checkbox")
	}
	if app.Find("id:field-label").Props.Role == core.RoleButton {
		t.Fatal("label became a button")
	}
}

func TestVisibleLabelActivatesSystemSwitch(t *testing.T) {
	if target := nsgotest.Target(); target != "macos" && target != "ios" && target != "ios_simulator" {
		t.Skip("requires an Apple target's native switch")
	}
	var scope *core.Scope
	var on, off *core.Signal[bool]
	ref := new(core.Ref)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		on, off = core.NewSignal(cx, false), core.NewSignal(cx, false)
		return core.View{Style: core.Style{Padding: core.All(20), Gap: 8}, Children: []core.Node{
			core.Text{TestID: "label", Content: "Updates", LabelFor: ref, Style: core.Style{Height: core.Pt(32)}},
			core.SystemSwitch{Ref: ref, TestID: "switch", Checked: on.Get, Disabled: off.Get, OnChange: on.Set},
			core.Text{TestID: "state", ContentFunc: func() string {
				if on.Get() {
					return "on"
				}
				return "off"
			}},
		}}
	}, nsgotest.Size(320, 200))
	app.Press("id:label")
	if app.Find("id:state").Text != "on" {
		t.Fatal("label did not activate native switch")
	}
	scope.Post(func() { off.Set(true) })
	app.Advance(0)
	if !app.Find("id:label").Props.Disabled {
		t.Fatal("label did not follow native switch's disabled state")
	}
	scope.Post(func() { off.Set(false) })
	app.Advance(0)
	app.Press("id:label")
	if app.Find("id:state").Text != "off" {
		t.Fatal("re-enabled label did not activate native switch")
	}
}
