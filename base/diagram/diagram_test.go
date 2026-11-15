package diagram

import (
	"slices"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/i18n"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

const flowchart = "flowchart LR\n  A[Start] --> B{Choice}\n  B -->|yes| C[Done]"

// texts are what a canvas's drawing writes, in order.
func texts(v *nsgotest.View) []string {
	var out []string
	for _, op := range v.Ops {
		if op.Kind == driver.DrawFillText {
			out = append(out, op.Text)
		}
	}
	return out
}

// colors are the colors a canvas's drawing sets, as fills and strokes.
func colors(v *nsgotest.View) map[core.Color]bool {
	out := map[core.Color]bool{}
	for _, op := range v.Ops {
		if op.Kind == driver.DrawFillColor || op.Kind == driver.DrawStrokeColor {
			out[op.Color] = true
		}
	}
	return out
}

func TestViewLaysOutAndPaints(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignStretch}, Children: []core.Node{View{TestID: "d", Source: flowchart}}}
	}, nsgotest.Size(800, 600))
	d := app.Find("id:d")
	if d.Kind != driver.KindCanvas {
		t.Fatalf("the diagram shows as %v", d.Kind)
	}
	size := d.Frame.Size
	if size.Width <= size.Height || size.Height <= 0 {
		t.Errorf("a left-to-right flowchart is %v: wider than tall, and not stretched by its column", size)
	}
	if got := texts(d); !slices.Contains(got, "Start") || !slices.Contains(got, "Choice") || !slices.Contains(got, "Done") || !slices.Contains(got, "yes") {
		t.Errorf("the drawing writes %q, want every node's and edge's label", got)
	}
	strokes := 0
	for _, op := range d.Ops {
		if op.Kind == driver.DrawStroke {
			strokes++
		}
	}
	if strokes < 5 {
		t.Errorf("%d strokes, want the nodes' outlines and the edges", strokes)
	}
	if got := d.Props.Label; got != "Diagram" {
		t.Errorf("label %q, want what a diagram is called without a title", got)
	}
	app.Screenshot("diagram-flowchart")
}

// A label's spans are drawn in their faces: a bold span in bold, as it was measured.
func TestLabelsDrawInTheirFaces(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "d", Source: "flowchart LR\n  A[\"<b>bold</b> and <i>italic</i>\"]"}
	})
	var bold, italic bool
	for _, op := range app.Find("id:d").Ops {
		if op.Kind == driver.DrawFillText {
			bold = bold || strings.TrimSpace(op.Text) == "bold" && op.Font.Weight == core.FontWeightBold
			italic = italic || strings.TrimSpace(op.Text) == "italic" && op.Font.Italic
		}
	}
	if !bold || !italic {
		t.Errorf("bold drawn bold %v, italic drawn italic %v: %q", bold, italic, texts(app.Find("id:d")))
	}
}

func TestViewReadsItsTitleInTheAppsLanguage(t *testing.T) {
	i18n.SetApp(i18n.NewCatalog("en", "zh-Hans"))
	defer i18n.SetApp(nil)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			View{TestID: "plain", Source: flowchart},
			View{TestID: "titled", Source: "flowchart LR\n  accTitle: Sign in\n  accDescr: How a user signs in\n  A --> B"},
			View{TestID: "labelled", Source: flowchart, Label: "Choices"},
		}}
	}, nsgotest.Environment(core.Environment{Locale: "en-US"}))
	if got := app.Find("id:titled").Props.Label; got != "Sign in: How a user signs in" {
		t.Errorf("a titled diagram reads %q", got)
	}
	if got := app.Find("id:labelled").Props.Label; got != "Choices" {
		t.Errorf("a labelled diagram reads %q", got)
	}
	app.SetEnvironment(core.Environment{Locale: "zh-CN"})
	if got := app.Find("id:plain").Props.Label; got != "图表" {
		t.Errorf("an untitled diagram reads %q in Chinese", got)
	}
	if got := app.Find("id:titled").Props.Label; got != "Sign in：How a user signs in" {
		t.Errorf("a titled diagram reads %q in Chinese", got)
	}
}

// The diagram colors itself as the app shows, light or dark, or with the Theme it is given.
func TestViewFollowsTheAppearanceAndItsTheme(t *testing.T) {
	border := core.RGB(0x12ab34)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			View{TestID: "d", Source: flowchart},
			View{TestID: "themed", Source: flowchart, Theme: &Theme{Background: core.RGB(0xffffff), Text: core.RGB(0x000000), Muted: core.RGB(0x666666),
				Line: core.RGB(0x333333), Surface: core.RGB(0xeeeeee), SurfaceAlt: core.RGB(0xdddddd), Border: border,
				Accent: core.RGB(0x0000ff), Critical: core.RGB(0xff0000), Radius: 4, StrokeWidth: 1}},
		}}
	}, nsgotest.Environment(core.Environment{Dark: false}))
	light := colors(app.Find("id:d"))
	if !colors(app.Find("id:themed"))[border] {
		t.Error("a themed diagram does not stroke its shapes in the theme's border color")
	}
	app.SetEnvironment(core.Environment{Dark: true})
	dark := colors(app.Find("id:d"))
	same := true
	for c := range dark {
		same = same && light[c]
	}
	if same {
		t.Error("the diagram draws the same colors in dark as in light")
	}
}

func TestViewFallsBackToItsSource(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			View{TestID: "bad", Source: "not a diagram", ErrorStyle: core.Style{Color: core.RGB(0xff0000)}},
			View{TestID: "empty"},
		}}
	})
	v := app.Find("text:not a diagram")
	if v.TextStyle.Color != core.RGB(0xff0000) {
		t.Errorf("the source shows in %v, want ErrorStyle's color", v.TextStyle.Color)
	}
	if app.Find("id:empty").Kind != driver.KindText {
		t.Error("an empty diagram draws a canvas")
	}
}

// Each kind of diagram limu reads lays out and draws: the painter is the same for all of them.
func TestEveryKindDraws(t *testing.T) {
	sources := map[string]string{
		"sequence": "sequenceDiagram\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi",
		"class":    "classDiagram\n  Animal <|-- Duck\n  Animal : +int age",
		"state":    "stateDiagram-v2\n  [*] --> Still\n  Still --> [*]",
		"er":       "erDiagram\n  CUSTOMER ||--o{ ORDER : places",
		"gantt":    "gantt\n  title Plan\n  dateFormat YYYY-MM-DD\n  section A\n  Task :a1, 2026-01-01, 3d",
		"pie":      "pie title Pets\n  \"Dogs\" : 386\n  \"Cats\" : 85",
		"mindmap":  "mindmap\n  root((NSGo))\n    Go\n    TSX",
		"timeline": "timeline\n  2025 : NSH\n  2026 : NSGo",
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var kids []core.Node
		for id, src := range sources {
			kids = append(kids, View{TestID: id, Source: src})
		}
		return core.Scroll{Children: kids}
	}, nsgotest.Size(800, 600))
	for id := range sources {
		v := app.Find("id:" + id)
		if v.Kind != driver.KindCanvas || len(v.Ops) == 0 || v.Frame.Size.Width <= 0 {
			t.Errorf("the %s diagram shows as %v with %d operations, %v", id, v.Kind, len(v.Ops), v.Frame.Size)
		}
	}
}

// RenderDiagram draws a text view's mermaid code blocks, which TextView centers in a sideways scroll and copies as their fenced source; other code blocks, and a mermaid block still streaming in, show as code.
func TestRenderDiagram(t *testing.T) {
	md := "Steps:\n\n```mermaid\n" + flowchart + "\n```\n\n```go\nx := 1\n```"
	var streaming *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		streaming = core.NewSignal(cx, false)
		return core.Show{When: streaming.Get,
			Then: func(cx *core.Scope) core.Node {
				return textview.TextView{TestID: "doc", RenderDiagram: RenderDiagram, Markdown: "```mermaid\n" + flowchart, StreamingCaret: core.Text{Content: "|"}}
			},
			Else: func(cx *core.Scope) core.Node {
				return textview.TextView{TestID: "doc", RenderDiagram: RenderDiagram, Markdown: md}
			}}
	}, nsgotest.Size(400, 600))
	app.Screenshot("diagram-textview")
	label := app.Find("label:Diagram")
	text := label
	for text.Kind != driver.KindText {
		text = text.Parent
	}
	if want := "```mermaid\n" + flowchart + "\n```"; text.Spans[0].Copy != want {
		t.Errorf("the diagram copies as %q, want %q", text.Spans[0].Copy, want)
	}
	if text.TextStyle.Align != core.TextAlignCenter || text.Parent.Kind != driver.KindScroll || !text.Parent.Scroll.Horizontal {
		t.Errorf("the diagram's text aligns %v in a %v: it centers in a sideways scroll", text.TextStyle.Align, text.Parent.Kind)
	}
	if !app.Exists("text:x := 1") {
		t.Error("a go code block does not show its code")
	}
	streaming.Set(true)
	app.Settle()
	if app.Exists("label:Diagram") {
		t.Error("a mermaid block streaming in draws before it is complete")
	}
	if !app.Exists("text:" + strings.SplitN(flowchart, "\n", 2)[0]) {
		t.Errorf("a mermaid block streaming in does not show its code:\n%s", app.Dump())
	}
}
