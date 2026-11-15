package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestInput sizes a field and a multiline one, and checks a disabled
// field.
func TestInput(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				Input{Label: "Email", TestID: "email", Placeholder: "name@example.com"},
				Input{Label: "Notes", TestID: "notes", Multiline: true},
				Input{Label: "Off", TestID: "off", Disabled: func() bool { return true }},
			}}
		})
		if f := app.Find("email"); f.Frame.Size.Height != ButtonHeight || f.Style.BorderColor != p.FieldBorder {
			t.Fatalf("field = %v tall, edge %x; want %v and the field border", f.Frame.Size.Height, f.Style.BorderColor, ButtonHeight)
		}
		if got := app.Find("email").Input.CaretColor; got != p.Accent {
			t.Fatalf("caret = %x, want the accent", got)
		}
		if h := app.Find("notes").Frame.Size.Height; h <= ButtonHeight {
			t.Fatalf("multiline = %v tall, want more than a line", h)
		}
		if !app.Find("off").Input.Disabled {
			t.Fatal("the disabled field takes input")
		}
	})
}

func TestInputMergesCallerStyle(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Width: core.Pt(240)}, Children: []core.Node{
				Input{TestID: "styled", Placeholder: "Search", Style: core.Style{
					Height: core.Pt(24), FontSize: 12, Padding: core.Edges{Left: 10, Right: 28},
				}, StyleFunc: func() core.Style { return core.Style{FontFamily: "Menlo"} }},
			}}
		})
		f := app.Find("styled")
		if f.Frame.Size.Height != 24 {
			t.Fatalf("height = %v, want 24", f.Frame.Size.Height)
		}
		if f.Input.Style.Font.Size != 12 {
			t.Fatalf("font size = %v, want 12", f.Input.Style.Font.Size)
		}
		if right := f.Frame.Size.Width - f.Input.Content.Origin.X - f.Input.Content.Size.Width; right < 28 {
			t.Fatalf("right inset = %v, want at least 28", right)
		}
		if f.Input.Style.Font.Family != "Menlo" {
			t.Fatalf("font family = %q, want Menlo", f.Input.Style.Font.Family)
		}
	})
}

func TestComboboxKeepsChevronInset(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Width: core.Pt(240)}, Children: []core.Node{
				Combobox[string]{TestID: "fruit", Label: "Fruit", Options: []Option[string]{{Value: "a", Label: "Apple"}}},
			}}
		})
		var field *nsgotest.View
		var walk func(*nsgotest.View)
		walk = func(v *nsgotest.View) {
			for _, child := range v.Children {
				if child.Kind == driver.KindInput {
					field = child
					return
				}
				walk(child)
			}
		}
		walk(app.Find("fruit"))
		if field == nil {
			t.Fatal("combobox has no input field")
		}
		if right := field.Frame.Size.Width - field.Input.Content.Origin.X - field.Input.Content.Size.Width; right < 28 {
			t.Fatalf("text runs under the chevron: right inset %v", right)
		}
	})
}
