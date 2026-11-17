package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestTooltip shows a tooltip as the pointer rests on its trigger.
func TestTooltip(t *testing.T) {
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignStart, Padding: core.All(40)}, Children: []core.Node{
			Tooltip{Text: "Shared with 3 people", Trigger: core.Text{Content: "Shared"}, TestID: "shared"},
		}}
	})
	app.Hover("shared")
	app.Advance(600 * time.Millisecond)
	app.Find("Shared with 3 people")
}

func TestTooltipDescribesItsButtonWithoutAnAlert(t *testing.T) {
	app, _ := start(t, desktopEnv, func(*core.Scope) core.Node {
		return Tooltip{Text: "Writes the current document", Trigger: base.Button{Label: "Save", TestID: "save",
			Style: core.Style{Height: core.Pt(30)}, OnPress: func(core.PressEvent) {}}}
	})
	if got := app.Find("id:save").Props.Description; got != "Writes the current document" {
		t.Fatalf("button description before focus = %q", got)
	}
	app.Key("tab")
	assertTooltipHidden(t, app.Find("text:Writes the current document"))
}

func TestIconButtonKeyboardTooltipsDoNotRepeatTheirLabels(t *testing.T) {
	for _, c := range []struct {
		name    string
		variant IconButtonVariant
	}{{"ghost", IconButtonGhost}, {"primary", IconButtonPrimary}, {"raised", IconButtonRaised}} {
		t.Run(c.name, func(t *testing.T) {
			app, _ := start(t, desktopEnv, func(*core.Scope) core.Node {
				return IconButton{Icon: lucide.Trash, Label: "Delete", Variant: c.variant, TestID: "delete", OnPress: func() {}}
			})
			app.Key("tab")
			button := app.Find("id:delete")
			if !button.Focused || button.Props.Label != "Delete" || button.Props.Description != "" || button.Props.ElementsHidden {
				t.Fatalf("focused button props = %+v", button.Props)
			}
			assertTooltipHidden(t, app.Find("text:Delete"))
		})
	}
}

func assertTooltipHidden(t *testing.T, text *nsgotest.View) {
	t.Helper()
	for v := text; ; v = v.Parent {
		if v == nil {
			t.Fatal("the tooltip has no overlay")
		}
		if !v.Props.ElementsHidden || v.Props.Accessible != core.AccessibleNo || v.Props.LiveRegion != core.LiveRegionNone || v.Props.Role != core.RoleNone {
			t.Fatalf("tooltip exposes screen-reader content: %+v", v.Props)
		}
		if v.Props.PointerEvents == core.PointerEventsNone {
			return
		}
	}
}
