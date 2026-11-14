package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/core"
)

// TestTextView draws Markdown in the theme, under the app's own styles.
func TestTextView(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
				TextView{TestID: "doc", Markdown: "# Title\n\nSome `code` and a [link](https://example.com).\n\n```go\nfmt.Println()\n```\n"},
				TextView{TestID: "custom", Markdown: "# Custom", Styles: textview.Styles{textview.PartHeading1: {Color: core.RGB(0x3366cc)}}},
			}}
		})
		title := app.Find("doc >> Title")
		if title.TextStyle.Font.Size != TypeHeading.FontSize+4 || title.TextStyle.Color != p.Text {
			t.Fatalf("heading = %v %x, want the heading's size and the text color", title.TextStyle.Font.Size, title.TextStyle.Color)
		}
		if got := app.Find("doc >> fmt.Println()").TextStyle.Font.Family; got != core.FontMonospace {
			t.Fatalf("code = %q, want monospace", got)
		}
		if got := app.Find("custom >> Custom").TextStyle.Color; got != core.RGB(0x3366cc) {
			t.Fatalf("the app's heading = %x, want its own color", got)
		}
	})
}

// A typeset display formula is as large as the text around it, while its TeX alone shows in the code type.
func TestTextViewMathBlock(t *testing.T) {
	app, _ := start(t, core.Environment{}, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
			TextView{TestID: "typeset", Markdown: "$$\nx\n$$\n", RenderMath: func(cx *core.Scope, m textview.Math) core.Node {
				return core.Canvas{TestID: "formula", Em: core.EmBox{Width: 1, Ascent: 1}, Style: core.Style{AlignSelf: core.AlignStart}}
			}},
			TextView{TestID: "source", Markdown: "$$\nx\n$$\n"},
		}}
	})
	if got := app.Find("id:formula").Frame.Size.Height; got != TypeBody.FontSize {
		t.Errorf("a display formula is %v high, want the body's %v", got, TypeBody.FontSize)
	}
	if got := app.Find("source >> x").TextStyle.Font.Size; got != TypeCode.FontSize {
		t.Errorf("a formula's TeX is %v, want the code's %v", got, TypeCode.FontSize)
	}
}
