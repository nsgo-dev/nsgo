package textview_test

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

var (
	hitColor      = core.RGB(0xffee00)
	selectedColor = core.RGB(0xff8800)
	searchStyles  = textview.Styles{textview.PartSearch: {Background: hitColor}, textview.PartSearchSelected: {Background: selectedColor}}
)

// highlights are the characters the texts show highlighted, in tree order: "*" before the selected match's.
func highlights(t *testing.T, app *nsgotest.App) []string {
	t.Helper()
	var out []string
	for _, v := range app.Query(`text:/[\s\S]/`) {
		if v.Kind != nsgotest.KindText {
			continue
		}
		for _, sp := range v.Spans {
			s := v.Text[sp.Start:sp.End]
			if !utf8.ValidString(s) {
				t.Errorf("span %q of %q is not UTF-8", s, v.Text)
			}
			switch sp.Background.Color {
			case hitColor:
				out = append(out, s)
			case selectedColor:
				out = append(out, "*"+s)
			}
		}
	}
	return out
}

// textIDs are the texts' views, which a search must keep for the selection to stay.
func textIDs(app *nsgotest.App) map[string]string {
	out := map[string]string{}
	for _, v := range app.Query(`text:/[\s\S]/`) {
		if v.Kind == nsgotest.KindText {
			out[v.Text] = fmt.Sprint(v.ID)
		}
	}
	return out
}

// background is the background of chars in the text holding text.
func background(t *testing.T, app *nsgotest.App, text, chars string) core.Color {
	t.Helper()
	v := app.Find("text:" + text)
	at := strings.Index(v.Text, chars)
	for _, sp := range v.Spans {
		if at >= sp.Start && at < sp.End {
			return sp.Background.Color
		}
	}
	t.Fatalf("no %q in %q", chars, v.Text)
	return 0
}

var mentionPattern = regexp.MustCompile(`@\w+`)

const searchDoc = "# README title\n\nSee[^1] 😀 readme and **READ**me.\n\n> a readme quote\n\n- item readme\n\n```go\nx := readme()\n```\n\n" +
	"| h | readme |\n|---|---|\n| readme cell | b |\n\n<details><summary>readme summary</summary>\n\nhidden readme\n\n</details>\n\n[^1]: footnote readme"

func TestTextViewSearchFindsWhatItShows(t *testing.T) {
	var s *textview.Search
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = textview.NewSearch(cx)
		return textview.TextView{Markdown: searchDoc, Styles: searchStyles, Search: s, Selectable: true,
			Highlighter: func(c textview.CodeBlock) []textview.Highlight {
				return []textview.Highlight{{Start: 5, End: 11, Style: core.Style{Color: core.RGB(0x0000ff)}}}
			}}
	}, nsgotest.Size(400, 2000))
	before := textIDs(app)
	// The search opens after the view is shown, as a find bar's does.
	s.SetQuery("readme")
	app.Settle()
	if s.Count() != 0 || len(highlights(t, app)) != 0 {
		t.Fatalf("a closed search found %d and shows %q", s.Count(), highlights(t, app))
	}
	s.Open()
	app.Settle()
	want := []string{"*README", "readme", "READ", "me", "readme", "readme", "readme", "readme", "readme", "readme", "readme"}
	if got := highlights(t, app); !reflect.DeepEqual(got, want) {
		t.Errorf("highlights %q, want %q", got, want)
	}
	if s.Count() != 10 || s.Selected() != 1 {
		t.Errorf("count %d, selected %d, want 10 and 1", s.Count(), s.Selected())
	}
	var paths []string
	for range s.Count() {
		m, ok := s.SelectedMatch()
		if !ok {
			t.Fatal("no selected match")
		}
		paths = append(paths, fmt.Sprint(m.Text.Footnote, m.Text.Path, m.From, m.To))
		s.Next()
	}
	// The paragraph's offsets are UTF-16 ones of its text, its footnote reference and emoji included.
	wantPaths := []string{"-1 [0] 0 6", "-1 [1] 10 16", "-1 [1] 21 27", "-1 [2 0] 2 8", "-1 [3 0 0] 5 11", "-1 [4] 5 11",
		"-1 [5 0 1] 0 6", "-1 [5 1 0] 0 6", "-1 [6 -1] 0 6", "0 [0] 9 15"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Errorf("matches %q, want %q", paths, wantPaths)
	}
	if s.Selected() != 1 {
		t.Errorf("Next did not wrap to the first: %d", s.Selected())
	}
	s.Previous()
	app.Settle()
	if s.Selected() != 10 || background(t, app, "footnote readme", "readme") != selectedColor {
		t.Errorf("Previous selected %d, the footnote's match %v", s.Selected(), background(t, app, "footnote readme", "readme"))
	}
	// The code's match keeps the highlighter's color under the search's.
	for range 4 {
		s.Previous()
	}
	app.Settle()
	if m, _ := s.SelectedMatch(); fmt.Sprint(m.Text.Path) != "[4]" {
		t.Fatalf("selected %v, want the code's", m)
	}
	code := app.Find("text:x := readme()")
	for _, sp := range code.Spans {
		if code.Text[sp.Start:sp.End] == "readme" && (sp.Background.Color != selectedColor || sp.Style.Color != core.RGB(0x0000ff)) {
			t.Errorf("the code's match %+v", sp)
		}
	}
	// A closed disclosure's content is searched once it opens.
	app.Press("text:readme summary")
	app.Settle()
	if s.Count() != 11 || !app.Exists("text:hidden readme") || background(t, app, "hidden readme", "readme") != hitColor {
		t.Errorf("opened, count %d", s.Count())
	}
	for range 4 {
		s.Next()
	}
	if m, _ := s.SelectedMatch(); fmt.Sprint(m.Text.Footnote, m.Text.Path) != "-1 [6 0]" {
		t.Errorf("the disclosure's paragraph is at %v", m.Text)
	}
	// Searching built no text again, so a selection stays; only the disclosure's marker changed.
	after := textIDs(app)
	for text, id := range before {
		if now, ok := after[text]; text != "▸" && (!ok || now != id) {
			t.Errorf("the search built %q again", text)
		}
	}
	if app.Find("text:item readme").Props.Selectable != core.SelectableOn {
		t.Error("a searched text is no longer selectable")
	}
	// Closing hides every match and keeps the query.
	s.Close()
	app.Settle()
	if s.Count() != 0 || len(highlights(t, app)) != 0 || s.Query() != "readme" {
		t.Errorf("closed: count %d, highlights %q, query %q", s.Count(), highlights(t, app), s.Query())
	}
	s.Open()
	app.Settle()
	if s.Count() != 11 {
		t.Errorf("opened again: count %d", s.Count())
	}
}

// The view finds again as the query, the options and the document change, and only what it shows.
func TestTextViewSearchFollowsChanges(t *testing.T) {
	var s *textview.Search
	var md *core.Signal[string]
	var pressed []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = textview.NewSearch(cx)
		md = core.NewSignal(cx, "Read [readme](https://x.y) $x$ read.\n\nHi @ada")
		return textview.TextView{MarkdownFunc: md.Get, Styles: searchStyles, Search: s,
			Extensions: []textview.Extension{{Name: "mention", Pattern: mentionPattern}},
			CustomSpan: func(n textview.Custom) core.Span { return core.Span{Content: "user " + n.Text[1:]} },
			RenderMath: func(cx *core.Scope, m textview.Math) core.Node {
				return core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)}}
			},
			OnLinkPress: func(e textview.LinkPressEvent) { pressed = append(pressed, e.URL) }}
	})
	s.Open()
	s.SetQuery("read")
	app.Settle()
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"*Read", "read", "read"}) {
		t.Errorf("highlights %q", got)
	}
	// The formula is an attachment, one character of the text, and the link still takes its press.
	if m := allMatches(s); fmt.Sprint(m) != "[[0] 0 4 [0] 5 9 [0] 14 18]" {
		t.Errorf("matches %v", m)
	}
	app.Press("text:readme")
	if !reflect.DeepEqual(pressed, []string{"https://x.y"}) {
		t.Errorf("pressed %q", pressed)
	}
	s.SetOptions(textview.FindOptions{WholeWord: true})
	app.Settle()
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"*Read", "read"}) {
		t.Errorf("whole words %q", got)
	}
	s.SetOptions(textview.FindOptions{CaseSensitive: true})
	s.SetQuery("user ada")
	app.Settle()
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"*user ada"}) {
		t.Errorf("the custom span's %q", got)
	}
	// A new document is found again, its old matches gone.
	md.Set("Nothing here.\n\nBut user ada.")
	app.Settle()
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"*user ada"}) || s.Count() != 1 {
		t.Errorf("after the change %q, count %d", got, s.Count())
	}
	md.Set("Nothing.")
	app.Settle()
	if s.Count() != 0 || s.Selected() != 0 {
		t.Errorf("no matches, count %d, selected %d", s.Count(), s.Selected())
	}
}

func allMatches(s *textview.Search) []any {
	var out []any
	for range s.Count() {
		m, _ := s.SelectedMatch()
		out = append(out, m.Text.Path, m.From, m.To)
		s.Next()
	}
	return out
}

// One search orders its views by the tree, wherever they register, and forgets those that go.
func TestSearchAcrossViews(t *testing.T) {
	var s *textview.Search
	var middle, last *core.Signal[bool]
	var md *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = textview.NewSearch(cx)
		textview.ProvideSearch(cx, s)
		middle, last = core.NewSignal(cx, false), core.NewSignal(cx, true)
		md = core.NewSignal(cx, "first readme")
		return core.Scroll{TestID: "scroll", Style: core.Style{Height: core.Pt(100)}, Children: []core.Node{
			textview.TextView{MarkdownFunc: md.Get, Styles: searchStyles},
			core.View{Style: core.Style{Height: core.Pt(400)}},
			core.Show{When: middle.Get, Then: func(*core.Scope) core.Node {
				return textview.TextView{Markdown: "middle readme", Styles: searchStyles}
			}},
			core.View{Style: core.Style{Height: core.Pt(400)}},
			core.Show{When: last.Get, Then: func(*core.Scope) core.Node {
				return textview.TextView{Markdown: "last readme\n\n```\n" + strings.Repeat("x", 300) + " readme\n```", Styles: searchStyles}
			}},
		}}
	}, nsgotest.Size(200, 300))
	s.SetQuery("readme")
	s.Open()
	app.Settle()
	if s.Count() != 3 || s.Selected() != 1 {
		t.Fatalf("count %d, selected %d", s.Count(), s.Selected())
	}
	// A view mounted late takes its place in reading order.
	middle.Set(true)
	app.Settle()
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"*readme", "readme", "readme", "readme"}) {
		t.Errorf("highlights %q", got)
	}
	s.Next()
	app.Settle()
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"readme", "*readme", "readme", "readme"}) {
		t.Errorf("after Next %q", got)
	}
	// Selecting a match reveals it, and SelectedBounds is its box on the surface.
	b, ok := s.SelectedBounds()
	if !ok || b.Origin.Y < 0 || b.Origin.Y+b.Size.Height > 100.5 {
		t.Errorf("selected bounds %v %v, not in the scroll view", b, ok)
	}
	if y := app.Find("id:scroll").Offset.Y; y == 0 {
		t.Error("the middle match was not revealed")
	}
	// A match in a long line of code is revealed through the code's own scroll view.
	s.Next()
	s.Next()
	app.Settle()
	if m, _ := s.SelectedMatch(); fmt.Sprint(m.Text.Path, m.From) != "[1] 301" {
		t.Fatalf("selected %v", m)
	}
	code := app.Find("text:" + strings.Repeat("x", 300)).Parent
	if code.Kind != nsgotest.KindScroll || code.Offset.X == 0 {
		t.Errorf("the code's scroll view %v at %v", code.Kind, code.Offset)
	}
	if b, ok := s.SelectedBounds(); !ok || b.Origin.X < 0 || b.Origin.X+b.Size.Width > 200.5 {
		t.Errorf("the code match's bounds %v %v", b, ok)
	}
	// The selected match stays while it exists, else the next after it is selected.
	md.Set("first readme, readme again")
	app.Settle()
	if m, _ := s.SelectedMatch(); s.Count() != 5 || fmt.Sprint(m.Text.Path, m.From) != "[1] 301" {
		t.Errorf("count %d, selected %v", s.Count(), m)
	}
	s.Previous()
	s.Previous()
	app.Settle()
	middle.Set(false)
	app.Settle()
	if got := highlights(t, app); s.Count() != 4 || !reflect.DeepEqual(got, []string{"readme", "readme", "*readme", "readme"}) {
		t.Errorf("after the middle view went: count %d, %q", s.Count(), got)
	}
	last.Set(false)
	app.Settle()
	if s.Count() != 2 || s.Selected() != 2 {
		t.Errorf("after the last view went: count %d, selected %d", s.Count(), s.Selected())
	}
}

// A reply streaming in keeps its selected match while its last block is built again, and finds what arrives.
func TestTextViewSearchFollowsAStream(t *testing.T) {
	var s *textview.Search
	var md *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = textview.NewSearch(cx)
		md = core.NewSignal(cx, "")
		return textview.TextView{MarkdownFunc: md.Get, Options: textview.Options{Streaming: true}, Styles: searchStyles, Search: s,
			StreamingCaret: core.View{TestID: "caret", Style: core.Style{Width: core.Pt(6), Height: core.Pt(12)}}}
	})
	s.SetQuery("readme")
	s.Open()
	reply := "Some readme text, and **another readme** to end."
	for i := range len(reply) + 1 {
		md.Set(reply[:i])
		app.Settle()
		if strings.Contains(reply[:i], "readme") && s.Selected() != 1 {
			t.Fatalf("at %q selected %d", reply[:i], s.Selected())
		}
	}
	if got := highlights(t, app); s.Count() != 2 || !reflect.DeepEqual(got, []string{"*readme", "readme"}) {
		t.Errorf("count %d, highlights %q", s.Count(), got)
	}
	if !app.Exists("id:caret") {
		t.Error("the caret went")
	}
}

// A formula's TeX is searched where the view shows it, not where RenderMath draws it.
func TestTextViewSearchFindsTeXItShows(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s := textview.NewSearch(cx)
		s.SetQuery("frac")
		s.Open()
		textview.ProvideSearch(cx, s)
		return core.View{Children: []core.Node{
			textview.TextView{Markdown: "Inline $\\frac{a}{b}$ text.\n\n$$\n\\frac{c}{d}\n$$", Styles: searchStyles},
			textview.TextView{Markdown: "Drawn $\\frac{a}{b}$ text.\n\n$$\n\\frac{c}{d}\n$$", Styles: searchStyles,
				RenderMath: func(cx *core.Scope, m textview.Math) core.Node {
					return core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)}}
				}},
		}}
	})
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"*frac", "frac"}) {
		t.Errorf("highlights %q", got)
	}
}

// A view keeps at most a thousand matches, and highlights only those it counts.
func TestTextViewSearchCapsItsMatches(t *testing.T) {
	var s *textview.Search
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = textview.NewSearch(cx)
		return textview.TextView{Markdown: strings.Repeat("a ", 700) + "\n\n" + strings.Repeat("a ", 700), Styles: searchStyles, Search: s}
	})
	s.SetQuery("a")
	s.Open()
	app.Settle()
	if n := len(highlights(t, app)); s.Count() != 1000 || n != 1000 {
		t.Errorf("count %d, %d highlighted", s.Count(), n)
	}
}

// A text's matches restyle it in place: its inline image, formula and caret keep their views as the query, the selection and the search's opening change.
func TestTextViewSearchKeepsAttachments(t *testing.T) {
	var s *textview.Search
	var formulas, carets int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s = textview.NewSearch(cx)
		return textview.TextView{Markdown: "See ![a](https://x.y/a.png) and $x$ readme, then readme.", Styles: searchStyles, Search: s,
			Options: textview.Options{Streaming: true},
			RenderMath: func(cx *core.Scope, m textview.Math) core.Node {
				formulas++
				return core.View{TestID: "formula", Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)}}
			},
			StreamingCaret: core.Func(func(cx *core.Scope) core.Node {
				carets++
				return core.View{TestID: "caret", Style: core.Style{Width: core.Pt(6), Height: core.Pt(12)}}
			})}
	})
	image := app.Find("role:image")
	ids := func() string {
		return fmt.Sprint(app.Find("role:image").ID, app.Find("id:formula").ID, app.Find("id:caret").ID)
	}
	before := ids()
	s.Open()
	for i := range len("readme") {
		s.SetQuery("readme"[:i+1])
		app.Settle()
	}
	if got := highlights(t, app); !reflect.DeepEqual(got, []string{"*readme", "readme"}) {
		t.Fatalf("highlights %q", got)
	}
	s.Next()
	app.Settle()
	s.Next()
	app.Settle()
	s.Close()
	app.Settle()
	s.Open()
	app.Settle()
	if formulas != 1 || carets != 1 {
		t.Errorf("the formula was made %d times and the caret %d, want once each", formulas, carets)
	}
	if got := ids(); got != before || app.Find("role:image") != image {
		t.Errorf("the attachments' views %s, want %s", got, before)
	}
}
