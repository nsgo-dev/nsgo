package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestButton presses each variant, 28 points tall on every device, and an
// icon's button as a square.
func TestButton(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var pressed []string
		variants := []ButtonVariant{ButtonPrimary, ButtonSecondary, ButtonOutline, ButtonGhost}
		names := []string{"primary", "secondary", "outline", "ghost"}
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			var row []core.Node
			for i, v := range variants {
				row = append(row, Button{Variant: v, Text: names[i], TestID: names[i], OnPress: func() { pressed = append(pressed, names[i]) }})
			}
			row = append(row,
				Button{Icon: lucide.Plus, Label: "Add", TestID: "icon", Variant: ButtonOutline},
				Button{Text: "Off", TestID: "disabled", Disabled: func() bool { return true }, OnPress: func() { pressed = append(pressed, "off") }})
			return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8, AlignItems: core.AlignStart}, Children: row}
		})
		for _, n := range names {
			app.Press(n)
			if h := app.Find(n).Frame.Size.Height; h != ButtonHeight {
				t.Fatalf("%s takes presses %v tall, want %v", n, h, ButtonHeight)
			}
			if h := app.Find(n).Children[0].Frame.Size.Height; h != ButtonHeight {
				t.Fatalf("%s face = %v tall, want %v", n, h, ButtonHeight)
			}
		}
		if len(pressed) != len(names) {
			t.Fatalf("pressed %v, want each variant", pressed)
		}
		if got := app.Find("primary").Children[0].Style.Background; got != p.AccentFill {
			t.Fatalf("primary = %x, want the accent", got)
		}
		if got := app.Find("ghost").Children[0].Style.Background; got != 0 {
			t.Fatalf("ghost = %x at rest, want no fill", got)
		}
		if f := app.Find("icon").Children[0].Frame.Size; f.Width != ButtonHeight || f.Height != ButtonHeight {
			t.Fatalf("icon's button = %v, want a %v square", f, ButtonHeight)
		}
		if app.Find("role:button Add").Props.Label != "Add" {
			t.Fatal("the icon's button has no label")
		}
		if !app.Find("disabled").Props.Disabled {
			t.Fatal("the disabled button is not reported disabled")
		}
	})
}

// TestButtonBusy shows a spinner in the text's color over the content, which keeps its room, while Busy returns true, reports the button busy and ignores its presses, and goes back once it turns false.
func TestButtonBusy(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		saves := 0
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			busy := core.NewSignal(cx, false)
			return core.View{Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignStart}, Children: []core.Node{
				Button{Text: "Save", TestID: "save", Busy: busy.Get, OnPress: func() { saves++ }},
				Button{Text: "Toggle", TestID: "toggle", Variant: ButtonSecondary, OnPress: func() { busy.Set(!busy.Get()) }},
			}}
		})
		idle := app.Find("save").Frame.Size
		spinning := func() bool { return len(canvasStrokes(app.Find("save"))) > 0 }
		if app.Find("save").Props.Busy || spinning() {
			t.Fatal("an idle button is busy")
		}
		app.Press("save")
		app.Press("toggle")
		save := app.Find("save")
		if !save.Props.Busy {
			t.Fatal("the busy button is not reported busy")
		}
		if got := canvasStrokes(save); len(got) != 2 || got[1] != p.OnAccent {
			t.Fatalf("busy spinner strokes %x, want its arc in %x", got, p.OnAccent)
		}
		if save.Frame.Size != idle {
			t.Fatalf("busy button is %v, want %v as when idle", save.Frame.Size, idle)
		}
		center := func(b core.Bounds) core.Point {
			return core.Point{X: b.Origin.X + b.Size.Width/2, Y: b.Origin.Y + b.Size.Height/2}
		}
		for _, v := range views(save) {
			if v.Kind == driver.KindCanvas {
				if got, want := center(v.Bounds()), center(save.Bounds()); got != want {
					t.Fatalf("spinner centered at %v, want the button's center %v", got, want)
				}
			}
		}
		app.Press("save")
		if saves != 1 {
			t.Fatalf("saved %d times, want once: a busy button takes no presses", saves)
		}
		app.Press("toggle")
		if app.Find("save").Props.Busy || spinning() {
			t.Fatal("the button stays busy once Busy turns false")
		}
		app.Press("save")
		if saves != 2 {
			t.Fatalf("saved %d times, want twice", saves)
		}
	})
}
