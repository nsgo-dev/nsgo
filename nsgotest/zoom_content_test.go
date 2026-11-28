package nsgotest

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestZoomContentUsesImageBounds(t *testing.T) {
	var ref core.Ref
	app := Start(t, func(*core.Scope) core.Node {
		return core.Scroll{TestID: "zoom", Ref: &ref, MinZoomScale: 1, MaxZoomScale: 4, CenterContent: true,
			Style:    core.Style{Width: core.Pt(300), Height: core.Pt(600)},
			Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(180), Height: core.Pt(600), FlexShrink: 0}}}}
	}, Size(300, 600))
	v := app.Find("zoom")
	if v.Content.Width != 180 || v.Content.Height != 600 {
		t.Fatalf("image bounds = %+v", v.Content)
	}
	app.Post(func() { ref.ZoomTo(2, nil, false); ref.ScrollTo(core.Point{X: 1000, Y: 1000}, false) })
	app.Settle()
	v = app.Find("zoom")
	if v.Offset.X != 60 || v.Offset.Y != 600 {
		t.Errorf("zoomed edge = %+v, want (60, 600)", v.Offset)
	}
}

func TestZoomContentIncludesVisibleOverflow(t *testing.T) {
	for _, overflow := range []core.Overflow{core.Visible, core.Hidden} {
		app := Start(t, func(*core.Scope) core.Node {
			return core.Scroll{TestID: "zoom", MinZoomScale: 1, MaxZoomScale: 4, CenterContent: true,
				Style: core.Style{Width: core.Pt(300), Height: core.Pt(600)},
				Children: []core.Node{core.View{
					Style:    core.Style{Width: core.Pt(180), Height: core.Pt(100), FlexShrink: 0, Overflow: overflow},
					Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(180), Height: core.Pt(900), FlexShrink: 0}}},
				}}}
		}, Size(300, 600))
		want := core.Points(900)
		if overflow == core.Hidden {
			want = 100
		}
		if got := app.Find("zoom").Content.Height; got != want {
			t.Errorf("overflow %v: content height = %v, want %v", overflow, got, want)
		}
	}
}
