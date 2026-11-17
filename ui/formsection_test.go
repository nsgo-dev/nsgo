package ui

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestFormSection divides the rows from the start of the titles above, has a
// pressable row press and a field edit its text.
func TestFormSection(t *testing.T) {
	headless(t)
	pressed := 0
	var text *core.Signal[string]
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		text = core.NewSignal(cx, "")
		return FormSection{Title: "Account", Footer: "Kept on this device.", TestID: "section", Children: []core.Node{
			FormField{Title: "Name", Input: base.Input{Value: text.Get, OnChange: text.Set, Placeholder: "Ada"}},
			FormRow{Icon: lucide.Bell, Title: "Alerts", Value: func() string { return "On" }, OnPress: func() { pressed++ }, TestID: "alerts"},
			FormRow{Title: "Sign out", Variant: FormRowDanger, OnPress: func() { pressed++ }},
		}}
	})
	card := app.Find("id:section").Children[1]
	if got := cardRows(card); got != "r|r|r" {
		t.Fatalf("card holds %q, want 3 rows and 2 hairlines", got)
	}
	if x := card.Children[1].Frame.Origin.X; x != formRowPad {
		t.Errorf("hairline below a field starts at %v, want %v", x, formRowPad)
	}
	if x, want := card.Children[3].Frame.Origin.X, formRowPad+formIconTile+formRowGap; x != want {
		t.Errorf("hairline below a row with an icon starts at %v, want %v", x, want)
	}
	app.Press("id:alerts")
	app.Press("Sign out")
	if pressed != 2 {
		t.Fatalf("pressed %d rows, want 2", pressed)
	}
	app.Type("label:Name", "Grace")
	if text.GetUntracked() != "Grace" {
		t.Fatalf("field holds %q", text.GetUntracked())
	}
}

// TestFormSectionDividesRowsOfEach gives each row from core.Each the hairline below it, from where its title starts, and keeps the card's last view free of one, as rows come, go and move, and as an Each next to rows of their own empties.
func TestFormSectionDividesRowsOfEach(t *testing.T) {
	headless(t)
	var list, more *core.Signal[[]string]
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		list, more = core.NewSignalFunc(cx, []string{"one", "two"}, nil), core.NewSignalFunc(cx, []string(nil), nil)
		row := func(cx *core.Scope, item func() string, _ func() int) core.Node {
			if item() == "bell" {
				return FormRow{Icon: lucide.Bell, Title: item()}
			}
			return FormRow{Title: item()}
		}
		key := func(s string) string { return s }
		return core.View{Children: []core.Node{
			FormSection{TestID: "each", Children: []core.Node{core.Each(list.Get, key, row)}},
			FormSection{TestID: "mixed", Children: []core.Node{
				FormRow{Title: "Top"}, core.Each(more.Get, key, row), FormRow{Title: "Add"},
			}},
		}}
	})
	card := func(id string) *nsgotest.View { return app.FindMounted("id:" + id).Children[0] }
	check := func(step, id, want string) {
		t.Helper()
		app.Settle()
		if got := cardRows(card(id)); got != want {
			t.Errorf("%s: %s card holds %q, want %q", step, id, got, want)
		}
	}
	check("start", "each", "r|r")
	check("start", "mixed", "r|r")
	list.Set([]string{"one", "two", "three"})
	check("append", "each", "r|r|r")
	list.Set([]string{"two", "three"})
	check("remove the first", "each", "r|r")
	list.Set([]string{"three", "bell", "two"})
	check("insert and move", "each", "r|r|r")
	if x, want := card("each").Children[3].Frame.Origin.X, formRowPad+formIconTile+formRowGap; x != want {
		t.Errorf("hairline below a row with an icon starts at %v, want %v", x, want)
	}
	if x := card("each").Children[1].Frame.Origin.X; x != formRowPad {
		t.Errorf("hairline below a row without one starts at %v, want %v", x, formRowPad)
	}
	list.Set([]string{"two", "three", "bell"})
	check("move the last", "each", "r|r|r")
	if x := card("each").Children[3].Frame.Origin.X; x != formRowPad {
		t.Errorf("after the move the hairline below the second row starts at %v, want %v", x, formRowPad)
	}
	list.Set(nil)
	check("empty", "each", "")
	more.Set([]string{"one"})
	check("fill the middle", "mixed", "r|r|r")
	more.Set(nil)
	check("empty the middle", "mixed", "r|r")
}

// cardRows spells a card's views: r for a row, | for a hairline shown and nothing for one hidden.
func cardRows(card *nsgotest.View) string {
	var b strings.Builder
	for _, v := range card.Children {
		switch {
		case v.Style.Hidden:
		case v.Frame.Size.Height == 1:
			b.WriteByte('|')
		default:
			b.WriteByte('r')
		}
	}
	return b.String()
}
