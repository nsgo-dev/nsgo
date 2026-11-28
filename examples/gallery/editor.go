package main

import (
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/editor"
	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/base/textview/highlight"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
	uieditor "github.com/nsgo-dev/nsgo/ui/editor"
)

// base/editor's Editor draws its text, gutter, carets and matches itself
// and keeps its own model; what is around it is the app's, as in NSH's
// gallery: the find bar is the app's inputs passing a Search and calling
// the ref, the status line reads the editor's events. On the TypeScript
// sample a simulated language server (editor_lsp.go) answers what the
// editor asks (a hover, a definition, code actions, completions) a little
// later, as a real one does over its connection; the other samples carry
// a linter's fixed diagnostics. What the server answers is the app's to
// show: diagnostics and decorations are data the editor draws, the hover
// card is an Overlay with a TextView, the code actions a menu at the
// caret.

// editorSample is a sample file of a language, with a linter's problems.
type editorSample struct {
	label, file, text string
	diagnostics       []editor.Diagnostic
}

func at(line, start, end int) editor.Range {
	return editor.Range{Start: editor.Position{Line: line, Column: start}, End: editor.Position{Line: line, Column: end}}
}

// editorLanguages are the samples in the Language select's order.
var editorLanguages = []string{"typescript", "rust", "python", "json"}

var editorSamples = map[string]editorSample{
	// The language server's file: its diagnostics are the server's.
	"typescript": {label: "TypeScript", file: "orders.ts", text: lspSample},
	"rust": {label: "Rust", file: "main.rs", text: `use std::collections::HashMap;

/// Counts the words of a text, most frequent first.
fn word_counts(text: &str) -> Vec<(&str, usize)> {
    let mut counts: HashMap<&str, usize> = HashMap::new();
    for word in text.split_whitespace() {
        *counts.entry(word).or_insert(0) += 1;
    }
    let mut sorted: Vec<_> = counts.into_iter().collect();
    sorted.sort_by(|a, b| b.1.cmp(&a.1).then(a.0.cmp(b.0)));
    sorted
}

fn main() {
    let unused = 42;
    for (word, count) in word_counts("the quick fox jumps over the lazy dog the end") {
        println!("{word}: {count}");
    }
}
`, diagnostics: []editor.Diagnostic{{Range: at(14, 8, 14), Severity: editor.SeverityWarning, Message: "unused variable: `unused`",
		Source: "rustc", Code: "unused_variables"}}},
	"python": {label: "Python", file: "shapes.py", text: `from dataclasses import dataclass


@dataclass
class Point:
    x: float
    y: float

    def distance(self, other: "Point") -> float:
        return ((self.x - other.x) ** 2 + (self.y - other.y) ** 2) ** 0.5


def perimeter(points):
    total = 0
    for a, b in zip(points, points[1:] + points[:1]):
        total += a.distance(b)
    return total


print(perimeter([Point(0, 0), Point(3, 0), Point(3, 4)]))
`, diagnostics: []editor.Diagnostic{{Range: at(12, 14, 20), Severity: editor.SeverityInfo, Message: "Missing type annotation for `points`",
		Source: "mypy"}}},
	"json": {label: "JSON", file: "settings.json", text: `{
  "name": "gallery",
  "version": "0.1.0",
  "entry": "App.tsx",
  "tailwind": {
    "theme": "theme.json"
  },
  "capabilities": ["network", "storage"],
  "debug": tru
}
`, diagnostics: []editor.Diagnostic{{Range: at(8, 11, 14), Severity: editor.SeverityError, Message: "Value expected", Source: "json"}}},
}

// lspSample is the TypeScript sample, with what the simulated server
// finds in it: a TODO, a var, a loose ==, an unused variable, a
// misspelled method and two color literals.
const lspSample = "/** An order of the shop. */\ninterface Order {\n  id: number;\n  items: LineItem[];\n  coupon?: string;\n}\n\n" +
	"/** A product in an order, and how many of it. */\ninterface LineItem {\n  name: string;\n  price: number;\n  quantity: number;\n}\n\n" +
	"const BRAND = \"#ff8800\";\nconst MUTED = \"#6b7280\";\n\n" +
	"/** The lines of a receipt, printed to the console. */\nclass Receipt {\n  lines: string[] = [];\n  color: string = BRAND;\n\n" +
	"  /** Adds a line at the end. */\n  add(text: string) {\n    this.lines.push(text);\n  }\n\n" +
	"  /** Prints every line, one under the other. */\n  print() {\n    console.log(this.lines.join(\"\\n\"));\n  }\n}\n\n" +
	"/** The sum of the items' prices times their quantities. */\nfunction subtotal(order: Order): number {\n" +
	"  return order.items.reduce((sum, item) => sum + item.price * item.quantity, 0);\n}\n\n" +
	"/** What the order's coupon takes off `total`. */\nfunction discount(order: Order, total: number): number {\n" +
	"  // TODO: read the coupons from the server\n  if (order.coupon == \"SAVE10\") return total * 0.1;\n  return 0;\n}\n\n" +
	"/** The order's number and what it costs, coupon taken off. */\nfunction describe(order: Order): string {\n" +
	"  var total = subtotal(order);\n  const unused = order.id * 2;\n" +
	"  return `#${order.id}: ${(total - discount(order, total)).toFixed(2)}`;\n}\n\n" +
	"const order: Order = { id: 7, items: [{ name: \"Tea\", price: 4.5, quantity: 2 }] };\nconst label = describe(order);\n" +
	"const receipt = new Receipt();\nreceipt.add(label);\nreceipt.prnt();\n"

// editorHover is what the hover card shows.
type editorHover struct {
	anchor      core.Bounds
	markdown    string
	diagnostics []editor.Diagnostic
}

// quickFixMenu is the code actions' menu at the caret, on touch screens.
type quickFixMenu struct {
	anchor  core.Point
	actions []lspAction
}

// lspAnalysis is the server's diagnostics and colors of one text.
type lspAnalysis struct {
	problems []lspProblem
	colors   []lspColor
}

// pageTimer runs a function once a while after it is set, under the
// page's scope, replacing the one set before.
type pageTimer struct {
	cx   *core.Scope
	gen  int
	stop func() bool
}

func (t *pageTimer) after(d time.Duration, fn func()) {
	t.cancel()
	gen := t.gen
	t.cx.Post(func() {
		if gen == t.gen {
			t.stop = t.cx.AfterFunc(d, fn)
		}
	})
}

func (t *pageTimer) cancel() {
	t.gen++
	if t.stop != nil {
		t.stop()
		t.stop = nil
	}
}

func sameRange(a, b editor.Range) bool { return a == b }

// editorDemo is ui/editor's Editor beside its options: a sample file per
// language, the find and replace of its Search, the commands of its Ref
// and what its events report; a context menu of editing commands. On the
// TypeScript sample a simulated language server: diagnostics after each
// pause in typing, hover cards with the symbol's signature and docs, go to
// definition (Mod-hover underlines, Mod-click and F12 go), quick fixes and
// refactorings (Mod+.), the colors of hex literals, completion (as you
// type, after a dot, Ctrl+Space) and inline completion (Tab).
func editorDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	language := core.NewSignal(cx, "typescript")
	softWrap, noLineNumbers := core.NewSignal(cx, false), core.NewSignal(cx, false)
	whitespaces, readOnly := core.NewSignal(cx, false), core.NewSignal(cx, false)
	query, replacement := core.NewSignal(cx, ""), core.NewSignal(cx, "")
	caseSensitive, wholeWord, regex := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
	selections := core.NewSignalFunc(cx, []editor.Selection(nil), slices.Equal)
	result := core.NewSignal(cx, editor.SearchResult{Current: -1})
	last := core.NewSignal(cx, "—")
	// Each sample's text as last edited, so switching back finds it as it
	// was.
	texts := map[string]string{}
	ref, findField := &editor.Ref{}, &core.Ref{}
	server := &languageServer{}
	server.change(lspSample)
	unequal := func(a, b lspAnalysis) bool { return false }
	found := core.NewSignalFunc(cx, lspAnalysis{}, unequal)
	hover := core.NewSignalFunc(cx, (*editorHover)(nil), func(a, b *editorHover) bool { return a == b })
	// The declaration go to definition just went to, lit for a moment.
	flash := core.NewSignalFunc(cx, (*editor.Range)(nil), func(a, b *editor.Range) bool { return a == b })
	analyzing := core.NewSignal(cx, false)
	quickFixes := core.NewSignalFunc(cx, (*quickFixMenu)(nil), func(a, b *quickFixMenu) bool { return a == b })
	typescript := func() bool { return language.Get() == "typescript" }

	// The server's diagnostics and colors, asked for again after each
	// pause in typing. The server keeps the TypeScript text while another
	// sample shows, so its answers stay good for when it comes back.
	lint, closing, flashing := &pageTimer{cx: cx}, &pageTimer{cx: cx}, &pageTimer{cx: cx}
	version := 0
	analyze := func() {
		version++
		asked := version
		analyzing.Set(true)
		lspAsk(cx, server, 40*time.Millisecond, func(m *lspModel) lspAnalysis { return lspAnalysis{m.lint(), m.colors()} }, func(a lspAnalysis) {
			// An answer about an older text: a newer one is on its way.
			if asked != version {
				return
			}
			found.Set(a)
			analyzing.Set(false)
		})
	}
	analyze()
	changed := func(value string) {
		server.change(value)
		analyzing.Set(true)
		lint.after(300*time.Millisecond, analyze)
	}
	cx.Cleanup(func() { lint.cancel(); closing.cancel(); flashing.cancel() })

	// The problems of the sample shown: the server's, or the linter's
	// fixed ones, which have no fixes.
	problems := core.NewMemoFunc(cx, func() []lspProblem {
		if typescript() {
			return found.Get().problems
		}
		var out []lspProblem
		for _, d := range editorSamples[language.Get()].diagnostics {
			out = append(out, lspProblem{diagnostic: d})
		}
		return out
	}, func(a, b []lspProblem) bool { return reflect.DeepEqual(a, b) })
	diagnostics := func() []editor.Diagnostic {
		var out []editor.Diagnostic
		for _, p := range problems.Get() {
			out = append(out, p.diagnostic)
		}
		return out
	}
	count := func(s editor.Severity) int {
		n := 0
		for _, p := range problems.Get() {
			if p.diagnostic.Severity == s {
				n++
			}
		}
		return n
	}
	decorations := func() []editor.Decoration {
		if !typescript() {
			return nil
		}
		var out []editor.Decoration
		for _, c := range found.Get().colors {
			mark := core.Style{Background: c.color}
			out = append(out, editor.Decoration{Range: c.rng, Style: core.Style{Background: ui.Alpha(c.color, 0.25)},
				Gutter: &editor.GutterMark{Style: mark}, OverviewRuler: &editor.OverviewRulerMark{Style: mark}})
		}
		if lit := flash.Get(); lit != nil {
			out = append(out, editor.Decoration{Range: *lit, Style: core.Style{Background: ui.Alpha(colors.Accent(), 0.25)},
				LineStyle: core.Style{Background: ui.Alpha(colors.Accent(), 0.1)}})
		}
		return out
	}
	caret := func() string {
		sels := selections.Get()
		if len(sels) == 0 {
			return tr.EditorLn1Col1()
		}
		h := sels[len(sels)-1].Head
		s := tr.EditorLnCol(h.Line+1, h.Column+1)
		if len(sels) > 1 {
			s += " · " + tr.EditorCarets(len(sels))
		}
		return s
	}
	matches := func() string {
		r := result.Get()
		switch {
		case r.Error != "":
			return tr.EditorInvalidPattern()
		case query.Get() == "":
			return ""
		case r.Count == 0:
			return tr.EditorNoResults()
		case r.Current < 0:
			return tr.EditorMatches(r.Count)
		}
		return fmt.Sprintf("%d of %d", r.Current+1, r.Count)
	}

	// A code action: an editor command, or the server's edits applied as
	// one edit undo takes back in one step.
	apply := func(a lspAction) {
		if a.command != "" {
			ref.Run(a.command)
		}
		if len(a.edits) > 0 {
			ref.Edit(a.edits)
		}
		ref.Focus()
		last.Set(a.title)
	}
	showDefinition := func(t *lspTarget) {
		if t == nil {
			last.Set(tr.EditorNoDefinitionFound())
			return
		}
		p := t.rng.Start
		ref.SetSelections([]editor.Selection{{Anchor: p, Head: p}})
		ref.RevealPosition(p, true)
		lit := t.rng
		flash.Set(&lit)
		flashing.after(900*time.Millisecond, func() { flash.Set(nil) })
		last.Set(tr.EditorDefinitionAt(t.name, p.Line+1, p.Column+1))
	}
	definition := func(p editor.Position, then func(*lspTarget)) {
		lspAsk(cx, server, 30*time.Millisecond, func(m *lspModel) *lspTarget { return m.definition(p) }, then)
	}
	// menuItems is a menu of actions: the quick fixes, then the rest.
	menuItems := func(actions []lspAction) []desktop.MenuItem {
		var fixes, rest []desktop.MenuItem
		for _, a := range actions {
			item := desktop.MenuItem{Label: a.title, OnSelect: func() { apply(a) }}
			if a.kind == "quickfix" {
				fixes = append(fixes, item)
			} else {
				rest = append(rest, item)
			}
		}
		if len(fixes) > 0 && len(rest) > 0 {
			fixes = append(fixes, desktop.MenuItem{Separator: true})
		}
		return append(fixes, rest...)
	}
	contextMenu := func(p editor.Position, at core.Point) {
		if !desktop.IsAvailable(cx) {
			return
		}
		var fixes []lspAction
		if typescript() {
			for _, a := range codeActions(editor.Range{Start: p, End: p}, problems.GetUntracked()) {
				if a.kind == "quickfix" {
					fixes = append(fixes, a)
				}
			}
		}
		run := func(c editor.Command) func() { return func() { ref.Run(c) } }
		var items []desktop.MenuItem
		if typescript() {
			items = append(items, desktop.MenuItem{Label: tr.EditorGoToDefinition(), Shortcut: "f12", OnSelect: func() { definition(p, showDefinition) }},
				desktop.MenuItem{Separator: true})
		}
		if len(fixes) > 0 {
			items = append(append(items, menuItems(fixes)...), desktop.MenuItem{Separator: true})
		}
		items = append(items,
			desktop.MenuItem{Label: tr.EditorCut(), OnSelect: run(editor.Cut)},
			desktop.MenuItem{Label: tr.EditorCopy(), OnSelect: run(editor.Copy)},
			desktop.MenuItem{Label: tr.EditorPaste(), OnSelect: run(editor.Paste)},
			desktop.MenuItem{Label: tr.EditorSelectAll(), OnSelect: run(editor.SelectAll)},
			desktop.MenuItem{Separator: true},
			desktop.MenuItem{Label: tr.EditorToggleComment(), OnSelect: run(editor.ToggleComment)},
			desktop.MenuItem{Label: tr.EditorFoldAll(), OnSelect: run(editor.FoldAll)},
			desktop.MenuItem{Label: tr.EditorUnfoldAll(), OnSelect: run(editor.UnfoldAll)})
		desktop.PopupMenu(cx, items, at, nil)
	}

	// Completion has no default keys: the app picks them.
	cx.OnKey("ctrl+space", ref.ShowCompletions)

	// The hover card stays while the pointer is over it, so its text can
	// be selected and its fixes pressed: leaving the word closes it a
	// moment later unless the pointer went onto the card.
	hoverAsked := 0
	overCard := false
	closeSoon := func() {
		closing.after(250*time.Millisecond, func() {
			if !overCard {
				hover.Set(nil)
			}
		})
	}
	// openRule opens a diagnostic code's page (a lint rule's).
	openRule := func(href string) {
		cx.OpenURL(href, func(opened bool) {
			if !opened {
				last.Set(tr.EditorNoAppOpensLinks())
			}
		})
	}
	fixesFor := func(d editor.Diagnostic) []lspAction {
		for _, p := range problems.GetUntracked() {
			if p.diagnostic.Message == d.Message && sameRange(p.diagnostic.Range, d.Range) {
				return p.fixes
			}
		}
		return nil
	}
	// Mod+F puts the selection, when there is one, in the find field and
	// focuses it.
	cx.OnKey("mod+f", func() {
		if sels := selections.GetUntracked(); len(sels) > 0 && sels[len(sels)-1].Anchor != sels[len(sels)-1].Head {
			ref.Run(editor.FindWithSelection)
		}
		findField.Focus()
	})

	// source is the editor of the sample shown: switching remounts it, so
	// each keeps its undo history out of the others'.
	source := core.View{StyleFunc: func() core.Style {
		return ui.FinOf(cx).Card().Over(core.Style{FlexGrow: 1, FlexShrink: core.Shrink(1), MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset,
			BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Surface(), Overflow: core.Hidden})
	}, Children: []core.Node{
		core.Dynamic(func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			colors := ui.ColorsOf(cx)
			lang := language.Get()
			lsp := lang == "typescript"
			initial, ok := texts[lang]
			if !ok {
				initial = editorSamples[lang].text
			}
			e := uieditor.Editor{Ref: ref, TestID: "editor", Label: lang + tr.EditorSource(), DefaultValue: initial, Language: lang,
				SoftWrap: softWrap.Get(), NoLineNumbers: noLineNumbers.Get(), ShowWhitespaces: whitespaces.Get(), ReadOnly: readOnly.Get(),
				Diagnostics: diagnostics, Decorations: decorations,
				Search: func() editor.Search {
					return editor.Search{Query: query.Get(), CaseSensitive: caseSensitive.Get(), WholeWord: wholeWord.Get(), Regex: regex.Get()}
				},
				CompletionTriggerCharacters: []string{"."}, InlineCompletionDelay: 150 * time.Millisecond,
				OnChange: func(v string) {
					texts[lang] = v
					if lsp {
						changed(v)
					}
				},
				OnSelectionChange:   selections.Set,
				OnSearchChange:      result.Set,
				OnFindWithSelection: query.Set,
				OnLineNumberPress:   func(line int) { last.Set(tr.EditorLinePressed(line + 1)) },
				OnContextMenu:       func(e editor.ContextMenuEvent) { contextMenu(e.Position, e.At) },
				OnHover: func(e editor.HoverEvent) {
					closing.cancel()
					hoverAsked++
					asked := hoverAsked
					show := func(h *lspHover) {
						if asked != hoverAsked {
							return
						}
						if h == nil && len(e.Diagnostics) == 0 {
							hover.Set(nil)
							return
						}
						card := &editorHover{anchor: e.PageBounds, diagnostics: e.Diagnostics}
						if h != nil {
							card.markdown = h.markdown
						}
						hover.Set(card)
					}
					if !lsp {
						show(nil)
						return
					}
					lspAsk(cx, server, 25*time.Millisecond, func(m *lspModel) *lspHover { return m.hover(e.Position) }, show)
				},
				OnHoverEnd: func() {
					hoverAsked++
					closeSoon()
				},
				OnDefinitionRequest: func(r editor.DefinitionRequest) {
					if !lsp {
						if r.Trigger != editor.DefinitionHover {
							showDefinition(nil)
						}
						return
					}
					definition(r.Position, func(t *lspTarget) {
						// Mod held over a name: underlined when it has a
						// definition (the editor drops an answer to an
						// older request).
						if r.Trigger == editor.DefinitionHover {
							if t != nil {
								ref.PresentDefinitionLink(r.ID, r.Range)
							}
							return
						}
						showDefinition(t)
					})
				},
				OnCodeActionRequest: func(req editor.CodeActionRequest) {
					rng := req.Range
					box, ok := ref.CaretBounds()
					if !ok {
						return
					}
					anchor := core.Point{X: box.PageBounds.Origin.X, Y: box.PageBounds.Origin.Y + box.PageBounds.Size.Height}
					actions := codeActions(rng, problems.GetUntracked())
					later(cx, 20*time.Millisecond, func() {
						if desktop.IsAvailable(cx) {
							desktop.PopupMenu(cx, menuItems(actions), anchor, nil)
						} else {
							quickFixes.Set(&quickFixMenu{anchor: anchor, actions: actions})
						}
					})
				},
				OnCompletionAccept: func(e editor.CompletionAcceptEvent) { last.Set(tr.EditorCompleted() + e.Item.Label) },
				StyleFunc:          func() core.Style { return core.Style{FlexGrow: 1, Color: colors.Text()} },
			}
			if lsp {
				e.Completion = true
				e.OnCompletionRequest = func(r editor.CompletionRequest) {
					var latency time.Duration
					lspAsk(cx, server, 0, func(m *lspModel) []editor.CompletionItem {
						items, l := m.completion(r)
						latency = l
						return items
					}, func(items []editor.CompletionItem) {
						later(cx, latency, func() { ref.PresentCompletions(r.ID, items, false) })
					})
				}
				e.OnInlineCompletionRequest = func(r editor.InlineCompletionRequest) {
					lspAsk(cx, server, 50*time.Millisecond, func(m *lspModel) string { return m.inline(r.Position) }, func(s string) {
						if s != "" {
							ref.PresentInlineCompletion(r.ID, s)
						}
					})
				}
			}
			return e
		}),
	}}

	dot := func(s editor.Severity) core.Node {
		c := [...]func() core.Color{colors.Danger, colors.Warn, colors.Info, colors.Muted}[min(s, editor.SeverityHint)]
		return core.View{Style: core.Style{Margin: core.Edges{Top: 5}, Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4}, StyleFunc: ui.Filled(c)}
	}
	// hoverCard is the server's answer and the problems at the word, over
	// it: the problems with their sources, codes (a link to the rule's
	// page) and fixes, then the docs.
	hoverCard := core.Dynamic(func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		card := hover.Get()
		if card == nil {
			return nil
		}
		env := cx.Env()
		var children []core.Node
		for _, d := range card.diagnostics {
			meta := []core.Node{core.Text{Content: d.Source, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)}}
			switch {
			case d.Code != "" && d.CodeDescription != nil:
				href := d.CodeDescription.Href
				meta = append(meta, base.Button{Label: tr.EditorOpenTheDescriptionOf() + d.Code, OnPress: func(core.PressEvent) { openRule(href) },
					Style: core.Style{CornerRadius: 2, Active: &core.Style{Opacity: core.Alpha(0.7)}},
					Children: []core.Node{core.Text{Content: d.Code, Style: core.Style{FontSize: 12, TextDecoration: core.TextDecorationUnderline},
						StyleFunc: ui.Colored(colors.Info)}}})
			case d.Code != "":
				meta = append(meta, core.Text{Content: "(" + d.Code + ")", Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)})
			}
			var fixes []core.Node
			for _, fix := range fixesFor(d) {
				fixes = append(fixes, base.Button{Label: fix.title, OnPress: func(core.PressEvent) { apply(fix); hover.Set(nil) },
					StyleFunc: func() core.Style {
						return core.Style{CornerRadius: 6, Padding: core.Symmetric(4, 8), Background: ui.Alpha(colors.Accent(), 0.14),
							Hover: &core.Style{Background: ui.Alpha(colors.Text(), 0.15)}, Active: &core.Style{Opacity: core.Alpha(0.7)}}
					},
					Children: []core.Node{ui.Text(fix.title, 12, ui.Regular, colors.Text)}})
			}
			children = append(children, core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
					dot(d.Severity),
					core.View{Style: core.Style{FlexGrow: 1, FlexShrink: core.Shrink(1), Gap: 2}, Children: []core.Node{
						core.Text{Content: d.Message, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
						core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 4}, Children: meta},
					}},
				}},
				core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 6, Padding: core.Edges{Left: 16}}, Children: fixes},
			}})
		}
		if card.markdown != "" {
			docs := textview.TextView{Markdown: card.markdown, Selectable: true, StyleOf: readingStyles(cx), Highlighter: highlight.New(cx)}
			if len(card.diagnostics) > 0 {
				children = append(children, core.View{StyleFunc: func() core.Style {
					return core.Style{Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Border()}, Padding: core.Edges{Top: 8}}
				}, Children: []core.Node{docs}})
			} else {
				children = append(children, docs)
			}
		}
		anchor := card.anchor
		return core.Overlay{AnchorBounds: func() core.Bounds { return anchor }, Placement: core.PlaceTop, Align: core.AlignStart, Offset: 4, Margin: 8,
			Children: []core.Node{core.View{TestID: "hover-card",
				OnMouseEnter: func(core.MouseEvent) { overCard = true; closing.cancel() },
				OnMouseLeave: func(core.MouseEvent) { overCard = false; closeSoon() },
				// A width of its own: an overlay fits its content, which
				// would leave the message's growing text no room.
				StyleFunc: func() core.Style {
					return ui.FinOf(cx).Raised().Over(core.Style{Width: core.Pt(min(400, env.Window().Width-16)), Gap: 8, CornerRadius: 8, BorderWidth: 1,
						BorderColor: colors.Border(), Background: colors.Raised(), Padding: core.All(12)})
				}, Children: children}}}
	})

	// quickFixList is the code actions' menu at the caret where no native
	// menu shows.
	quickFixList := core.Dynamic(func(cx *core.Scope) core.Node {
		menu := quickFixes.Get()
		if menu == nil {
			return nil
		}
		dismiss := func() { quickFixes.Set(nil) }
		var rows []core.Node
		for _, a := range menu.actions {
			rows = append(rows, base.Button{Label: a.title, OnPress: func(core.PressEvent) { dismiss(); apply(a) },
				StyleFunc: func() core.Style {
					return core.Style{CornerRadius: 6, Padding: core.Symmetric(8, 12), Hover: &core.Style{Background: ui.Alpha(colors.Text(), 0.1)},
						Active: &core.Style{Background: ui.Alpha(colors.Text(), 0.15)}}
				},
				Children: []core.Node{ui.Text(a.title, 13, ui.Regular, colors.Text)}})
		}
		anchor := menu.anchor
		return core.Overlay{AnchorBounds: func() core.Bounds { return core.Bounds{Origin: anchor} }, Placement: core.PlaceBottom, Align: core.AlignStart,
			Margin: 8, OnPressOutside: func(core.Point) { dismiss() }, OnCancel: dismiss,
			Children: []core.Node{core.View{Role: core.RoleMenu, TestID: "quick-fixes", StyleFunc: func() core.Style {
				return ui.FinOf(cx).Raised().Over(core.Style{MinWidth: core.Pt(224), CornerRadius: 8, BorderWidth: 1, BorderColor: colors.Border(),
					Background: colors.Raised(), Padding: core.All(4)})
			}, Children: rows}}}
	})

	small := func(label string, onPress func()) core.Node {
		return ui.Button{Text: label, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: onPress}
	}
	run := func(c editor.Command) func() {
		return func() {
			ref.Run(c)
			ref.Focus()
		}
	}
	field := func(label string, r *core.Ref, value *core.Signal[string], submit func()) core.Node {
		return ui.Input{Ref: r, Label: label, Placeholder: label, Value: value.Get, OnChange: value.Set, OnSubmit: func(string) { submit() },
			AutoCapitalize: core.AutoCapitalizeNone, NoAutoCorrect: true}
	}
	status := func(fn func() string, c func() core.Color) core.Node {
		return core.Text{ContentFunc: fn, MaxLines: 1, Style: core.Style{FontFamily: ui.Mono, FontSize: 12, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(c)}
	}
	languages := make([]ui.Option[string], len(editorLanguages))
	for i, lang := range editorLanguages {
		languages[i] = ui.Option[string]{Value: lang, Label: editorSamples[lang].label}
	}
	replace := func() { ref.Replace(replacement.GetUntracked()) }

	source.Children = append(source.Children, hoverCard, quickFixList)
	return stagePage(cx, source,
		Section{Title: tr.TerminalOptions(), Note: "editor.Editor", Children: []core.Node{
			Knob{Name: "Language", Control: ui.Select[string]{Label: "Language", TestID: "knob-Language", Options: languages, Value: language.Get,
				OnChange: language.Set, Style: core.Style{Width: core.Pt(140)}}},
			SwitchRow{Label: tr.EditorSoftWrap(), Code: "SoftWrap", Checked: softWrap.Get, OnChange: softWrap.Set},
			SwitchRow{Label: tr.EditorHideLineNumbers(), Code: "NoLineNumbers", Checked: noLineNumbers.Get, OnChange: noLineNumbers.Set},
			SwitchRow{Label: tr.EditorShowWhitespace(), Code: "ShowWhitespaces", Checked: whitespaces.Get, OnChange: whitespaces.Set},
			SwitchRow{Label: tr.EditorReadOnly(), Code: "ReadOnly", Checked: readOnly.Get, OnChange: readOnly.Set},
		}},
		Section{Title: tr.EditorFindAndReplace(), Note: "editor.Search", Children: []core.Node{
			field(tr.EditorFind(), findField, query, run(editor.FindNext)),
			field(tr.EditorReplace(), nil, replacement, replace),
			SwitchRow{Label: tr.EditorMatchCase(), Code: "CaseSensitive", Checked: caseSensitive.Get, OnChange: caseSensitive.Set},
			SwitchRow{Label: tr.EditorWholeWord(), Code: "WholeWord", Checked: wholeWord.Get, OnChange: wholeWord.Set},
			SwitchRow{Label: tr.EditorRegularExpression(), Code: "Regex", Checked: regex.Get, OnChange: regex.Set},
			ui.Wrap{Children: []core.Node{
				small(tr.EditorPreviousMatch(), run(editor.FindPrevious)),
				small(tr.EditorNextMatch(), run(editor.FindNext)),
				small(tr.EditorReplace(), replace),
				small(tr.EditorReplaceAll(), func() { last.Set(tr.EditorReplaced(ref.ReplaceAll(replacement.GetUntracked()))) }),
			}},
			core.Show{When: func() bool { return query.Get() != "" }, Then: func(*core.Scope) core.Node { return status(matches, colors.Muted) }},
		}},
		Section{Title: tr.KitActions(), Note: "editor.Ref", Children: []core.Node{
			ui.Wrap{Children: []core.Node{
				small(tr.EditorFoldAll(), run(editor.FoldAll)),
				small(tr.EditorUnfoldAll(), run(editor.UnfoldAll)),
				small(tr.EditorToggleComment(), run(editor.ToggleComment)),
				small(tr.EditorSelectAll(), run(editor.SelectAll)),
			}},
		}},
		Section{Title: tr.TerminalStatus(), Note: "OnSelectionChange", Children: []core.Node{
			status(caret, colors.Text),
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 12}, Children: []core.Node{
				status(func() string { return tr.EditorErrors(count(editor.SeverityError)) }, colors.Danger),
				status(func() string { return tr.EditorWarnings(count(editor.SeverityWarning)) }, colors.Warn),
				status(func() string { return tr.EditorInfos(count(editor.SeverityInfo)) }, colors.Info),
			}},
			status(last.Get, colors.Muted),
		}})
}
