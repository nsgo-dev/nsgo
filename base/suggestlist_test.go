package base_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

var (
	suggestDesktop = core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, ScreenReader: true}
	suggestPhone   = core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone}
)

// suggestStage is an uncontrolled field with a SuggestList beside it, whose rows are a disabled one and the people a query starts, and another field.
type suggestStage struct {
	app       *nsgotest.App
	queries   []string
	submitted int
}

func startSuggestList(t *testing.T, env core.Environment) *suggestStage {
	t.Helper()
	st := &suggestStage{}
	people := []base.SuggestItem{{ID: "off", Label: "away", Disabled: true}, {ID: "al", Label: "alice", Insert: "@alice"}, {ID: "bo", Label: "bob"}}
	st.app = nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s := core.NewSuggest(cx, core.SuggestOptions{})
		open := core.NewSignal(cx, false)
		return core.View{Children: []core.Node{
			core.Input{TestID: "f", Label: "To", Suggest: s, Role: core.RoleCombobox, Expanded: open.Get, OnSubmit: func(string) { st.submitted++ }},
			base.SuggestList{Suggest: s, Label: "People", TestID: "people", PopupStyle: core.Style{Width: core.Pt(200)}, ItemStyle: core.Style{Height: core.Pt(30)}, OnOpen: func() { open.Set(true) }, OnClose: func() { open.Set(false) },
				Items: func(query string) []base.SuggestItem {
					st.queries = append(st.queries, query)
					if query == "none" {
						return nil
					}
					if query == "off" {
						return people[:1]
					}
					var out []base.SuggestItem
					for _, p := range people {
						if strings.HasPrefix(p.Label, query) || p.Disabled {
							out = append(out, p)
						}
					}
					return out
				}},
			core.Input{TestID: "other", Label: "Other"},
		}}
	}, nsgotest.Environment(env))
	return st
}

// TestSuggestListKeys moves the highlight over the enabled rows, Up and Down around the ends and the page keys stopping at them, announcing each row, and Return accepts it.
func TestSuggestListKeys(t *testing.T) {
	st := startSuggestList(t, suggestDesktop)
	app := st.app
	app.Type("id:f", "hi @")
	if got := app.Announced; len(got) == 0 || got[len(got)-1] != "alice" {
		t.Fatalf("announced %q as the rows showed", got)
	}
	for _, step := range []struct{ key, want string }{{"pagedown", "bob"}, {"pagedown", "bob"}, {"up", "alice"}, {"up", "bob"}, {"down", "alice"}, {"pageup", "alice"}, {"down", "bob"}} {
		app.Key(step.key)
		if got := app.Announced; got[len(got)-1] != step.want {
			t.Fatalf("%s: announced %q, want %q", step.key, got[len(got)-1], step.want)
		}
	}
	app.Key("enter")
	if v := app.Find("id:f"); v.Text != "hi bob" || v.Selection.Start != 6 || app.Exists("id:people") || st.submitted != 0 {
		t.Fatalf("text %q, selection %v, list shown %v, %d submits", v.Text, v.Selection, app.Exists("id:people"), st.submitted)
	}
	app.Key("enter")
	if st.submitted != 1 {
		t.Fatalf("Return after accepting: %d submits", st.submitted)
	}
}

// TestSuggestListTabInserts accepts the highlighted row's Insert with Tab, and the accepted trigger does not open the list again.
func TestSuggestListTabInserts(t *testing.T) {
	st := startSuggestList(t, suggestDesktop)
	app := st.app
	app.Type("id:f", "@a")
	app.Key("tab")
	if v := app.Find("id:f"); v.Text != "@alice" || app.Exists("id:people") {
		t.Fatalf("text %q, list shown %v", v.Text, app.Exists("id:people"))
	}
	if !app.Find("id:f").Focused {
		t.Fatal("Tab moved the focus")
	}
}

// TestSuggestListRoles has the field's Expanded follow the rows and the rows take the option role in one named listbox, taking no focus.
func TestSuggestListRoles(t *testing.T) {
	st := startSuggestList(t, suggestDesktop)
	app := st.app
	if app.Find("role:combobox To").Props.Expanded {
		t.Fatal("expanded before a query")
	}
	app.Type("id:f", "@")
	app.Find("role:listbox People")
	for _, label := range []string{"away", "alice", "bob"} {
		if !app.Find("role:option " + label).Props.NoFocus {
			t.Fatalf("row %s takes the focus", label)
		}
	}
	if !app.Find("role:option away").Props.Disabled {
		t.Fatal("the disabled row is enabled")
	}
	if !app.Find("role:combobox To").Props.Expanded {
		t.Fatal("not expanded while the rows show")
	}
	app.Key("escape")
	if app.Exists("id:people") || app.Find("role:combobox To").Props.Expanded {
		t.Fatal("Escape left the rows")
	}
}

// TestSuggestListLeavesTheFieldItsKeys gives Return to the field with no rows, with only disabled rows, after Escape and while an input method composes.
func TestSuggestListLeavesTheFieldItsKeys(t *testing.T) {
	st := startSuggestList(t, suggestDesktop)
	app := st.app
	app.Type("id:f", "@none")
	if app.Exists("id:people") || app.Find("id:f").Props.Expanded {
		t.Fatal("an empty result shows")
	}
	app.Key("enter")
	if st.submitted != 1 {
		t.Fatalf("no rows: %d submits", st.submitted)
	}
	app.Type("id:f", " @off")
	app.Find("role:option away")
	app.Key("enter")
	if st.submitted != 2 {
		t.Fatalf("only a disabled row: %d submits", st.submitted)
	}
	app.Type("id:f", " @")
	app.Compose("id:f", "a")
	if !app.Exists("id:people") {
		t.Fatal("a composition closed the list")
	}
	app.Key("enter")
	if v := app.Find("id:f"); st.submitted != 3 || strings.Contains(v.Text, "alice") {
		t.Fatalf("composing: %d submits, text %q", st.submitted, v.Text)
	}
	app.Compose("id:f", "")
	app.Type("id:f", " @")
	app.Find("id:people")
	app.Key("escape")
	app.Key("enter")
	if st.submitted != 4 {
		t.Fatalf("after Escape: %d submits", st.submitted)
	}
}

// TestSuggestListAsksOncePerQuery calls Items as the query changes, not as the caret moves within it or a value is the same.
func TestSuggestListAsksOncePerQuery(t *testing.T) {
	st := startSuggestList(t, suggestDesktop)
	app := st.app
	app.Type("id:f", "@a")
	app.Select("id:f", 2, 2)
	app.Type("id:f", "l")
	if want := []string{"", "a", "al"}; !slices.Equal(st.queries, want) {
		t.Fatalf("queries %q, want %q", st.queries, want)
	}
}

// TestSuggestListClosesWithTheFocus closes the rows as another field takes the focus.
func TestSuggestListClosesWithTheFocus(t *testing.T) {
	st := startSuggestList(t, suggestDesktop)
	app := st.app
	app.Type("id:f", "@")
	app.Type("id:other", "q")
	if app.Exists("id:people") {
		t.Fatal("the rows stayed")
	}
}

// TestSuggestListPressDown accepts a row on a desktop as the button goes down, before the press could move the focus.
func TestSuggestListPressDown(t *testing.T) {
	st := startSuggestList(t, suggestDesktop)
	app := st.app
	app.Type("id:f", "@")
	app.PressDown("role:option bob")
	if v := app.Find("id:f"); v.Text != "bob" || app.Exists("id:people") {
		t.Fatalf("text %q, list shown %v", v.Text, app.Exists("id:people"))
	}
}

// TestSuggestListTapOnAPhone accepts a tapped row and keeps the field's focus and the software keyboard: a phone's keyboard sends no arrow keys.
func TestSuggestListTapOnAPhone(t *testing.T) {
	st := startSuggestList(t, suggestPhone)
	app := st.app
	app.Type("id:f", "@")
	if taps := app.Find("role:listbox People").Scroll.Behavior.KeyboardShouldPersistTaps; taps != core.KeyboardTapsHandled {
		t.Fatalf("KeyboardShouldPersistTaps = %v", taps)
	}
	app.Press("role:option alice")
	if v := app.Find("id:f"); v.Text != "@alice" || app.Exists("id:people") || !v.Focused {
		t.Fatalf("text %q, list shown %v, field focused %v", v.Text, app.Exists("id:people"), v.Focused)
	}
}

// TestSuggestListAtTheCaret places the rows' overlay against the caret the field reports, below it unless placed otherwise.
func TestSuggestListAtTheCaret(t *testing.T) {
	var s *core.Suggest
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = core.NewSuggest(cx, core.SuggestOptions{})
		return core.View{Style: core.Style{Padding: core.Edges{Top: 300, Left: 40}}, Children: []core.Node{
			core.Input{TestID: "f", Label: "To", Suggest: s},
			base.SuggestList{Suggest: s, TestID: "people", Align: core.AlignStart, PopupStyle: core.Style{Width: core.Pt(200)},
				Items: func(string) []base.SuggestItem { return []base.SuggestItem{{ID: "al", Label: "alice"}} }},
		}}
	})
	app.Type("id:f", "hi @")
	caret := s.Anchor()
	if caret.Origin.Y < 300 || caret.Origin.X <= 40 {
		t.Fatalf("caret %v is not in the field", caret)
	}
	o := app.Find("id:people").Frame
	// The core lays the overlay out on whole points.
	near := func(a, b core.Points) bool { return a-b < 1 && b-a < 1 }
	if !near(o.Origin.Y, caret.Origin.Y+caret.Size.Height) || !near(o.Origin.X, caret.Origin.X) {
		t.Fatalf("overlay at %v, caret %v", o.Origin, caret)
	}
}
