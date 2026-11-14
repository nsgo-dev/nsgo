package formula

import (
	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/core"
)

// RenderMath shows a text view's formula with r, as textview.TextView.RenderMath takes it: an inline one on its text's baseline, and a display one centered on a line of its own, scrolling sideways where it is wider than the view. A copy takes them as their TeX between $ and $$.
func (r *Renderer) RenderMath(cx *core.Scope, m textview.Math) core.Node {
	if !m.Display {
		return View{Source: m.TeX, Renderer: r}
	}
	return core.Scroll{Horizontal: true, Children: []core.Node{
		core.Text{Style: core.Style{MinWidth: core.Pct(100), TextAlign: core.TextAlignCenter}, Spans: []core.Span{
			{Content: "$$\n" + m.TeX + "\n$$", Attachment: View{Source: m.TeX, Display: true, Renderer: r}},
		}},
	}}
}

// RenderMath is Default().RenderMath, for textview.TextView{RenderMath: formula.RenderMath}.
func RenderMath(cx *core.Scope, m textview.Math) core.Node { return Default().RenderMath(cx, m) }
