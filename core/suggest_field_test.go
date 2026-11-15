package core_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// suggestKeys stands for a list beside the field: Return accepts insert and Escape closes, while the query is open.
func suggestKeys(s *core.Suggest, insert string) core.Node {
	return core.Show{When: s.Open, Then: func(cx *core.Scope) core.Node {
		cx.OnKeyWith("enter", core.KeyOptions{When: core.KeyAlways}, func() { s.Accept(insert) })
		cx.OnKeyWith("escape", core.KeyOptions{When: core.KeyAlways}, s.Close)
		return nil
	}}
}

// startSuggestField starts an uncontrolled field with no Ref whose Suggest accepts insert.
func startSuggestField(t *testing.T, insert string) (*nsgotest.App, *core.Suggest) {
	t.Helper()
	var s *core.Suggest
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = core.NewSuggest(cx, core.SuggestOptions{})
		return core.View{Children: []core.Node{core.Input{TestID: "f", Label: "To", Suggest: s}, suggestKeys(s, insert)}}
	})
	return app, s
}

// TestSuggestAcceptsIntoAnUncontrolledField sets the text and the caret of a field with no Value and no Ref.
func TestSuggestAcceptsIntoAnUncontrolledField(t *testing.T) {
	app, s := startSuggestField(t, "alice")
	app.Type("id:f", "hi @al")
	if !s.Open() || s.Query() != "al" {
		t.Fatalf("open %v, query %q", s.Open(), s.Query())
	}
	app.Key("enter")
	v := app.Find("id:f")
	if s.Open() || v.Text != "hi alice" || v.Selection.Start != 8 || v.Selection.End != 8 {
		t.Fatalf("open %v, text %q, selection %v", s.Open(), v.Text, v.Selection)
	}
}

// TestSuggestStaysClosedOnAnAcceptedTrigger keeps the query closed on an insert that starts with the trigger, as the platform reports the caret the insert moves, until a trigger starts elsewhere.
func TestSuggestStaysClosedOnAnAcceptedTrigger(t *testing.T) {
	app, s := startSuggestField(t, "@alice")
	app.Type("id:f", "hi @al")
	app.Key("enter")
	if v := app.Find("id:f"); s.Open() || v.Text != "hi @alice" {
		t.Fatalf("open %v, text %q", s.Open(), v.Text)
	}
	app.Select("id:f", 7, 7)
	app.Select("id:f", 9, 9)
	app.Type("id:f", "x")
	if s.Open() {
		t.Fatal("the accepted trigger reopened the query")
	}
	app.Type("id:f", " @b")
	if !s.Open() || s.Query() != "b" {
		t.Fatalf("open %v, query %q", s.Open(), s.Query())
	}
	app.Key("escape")
	app.Select("id:f", 12, 12)
	app.Select("id:f", 13, 13)
	if s.Open() {
		t.Fatal("the dismissed trigger reopened the query")
	}
}

// TestSuggestClosesOnASelection closes the query while text is selected and opens it again as the caret returns to it.
func TestSuggestClosesOnASelection(t *testing.T) {
	app, s := startSuggestField(t, "alice")
	app.Type("id:f", "@al")
	app.Select("id:f", 0, 3)
	if s.Open() {
		t.Fatal("a selection left the query open")
	}
	app.Select("id:f", 3, 3)
	if !s.Open() || s.Query() != "al" {
		t.Fatalf("open %v, query %q", s.Open(), s.Query())
	}
	app.Select("id:f", 1, 1)
	if !s.Open() || s.Query() != "" {
		t.Fatalf("caret after the trigger: open %v, query %q", s.Open(), s.Query())
	}
	app.Select("id:f", 0, 0)
	if s.Open() {
		t.Fatal("a caret before the trigger left the query open")
	}
}

// TestSuggestOutlivesItsField leaves a Suggest whose field unmounted with a query open changing nothing as it accepts, and closes it on the field mounted again.
func TestSuggestOutlivesItsField(t *testing.T) {
	var s *core.Suggest
	var shown *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = core.NewSuggest(cx, core.SuggestOptions{})
		shown = core.NewSignal(cx, true)
		return core.View{Children: []core.Node{
			core.Show{When: shown.Get, Then: func(*core.Scope) core.Node { return core.Input{TestID: "f", Label: "To", Suggest: s} }},
			core.View{TestID: "toggle", Role: core.RoleButton, Label: "Toggle", Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) { shown.Set(!shown.GetUntracked()) }},
			core.View{TestID: "accept", Role: core.RoleButton, Label: "Accept", Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) { s.Accept("alice") }},
		}}
	})
	app.Type("id:f", "@al")
	if !s.Open() {
		t.Fatal("the query did not open")
	}
	app.Press("id:toggle")
	app.Press("id:accept")
	app.Press("id:toggle")
	if v := app.Find("id:f"); s.Open() || v.Text != "" {
		t.Fatalf("open %v, text %q", s.Open(), v.Text)
	}
	app.Type("id:f", "@b")
	if !s.Open() || s.Query() != "b" {
		t.Fatalf("open %v, query %q", s.Open(), s.Query())
	}
}
