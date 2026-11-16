package main

import (
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestStickyEndInsetsOnThePlatform checks the model and native pixels with
// the same app: a footer stays at the viewport's end while scrolling and
// flows immediately after content that fits, on either axis.
func TestStickyEndInsetsOnThePlatform(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		name := "bottom"
		if horizontal {
			name = "right"
		}
		t.Run(name, func(t *testing.T) {
			var scope *core.Scope
			var length, inset *core.Signal[core.Points]
			var pinned *core.Signal[bool]
			ref := new(core.Ref)
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				scope = cx
				pinned = core.NewSignal(cx, true)
				length, inset = core.NewSignal(cx, core.Points(600)), core.NewSignal(cx, core.Points(0))
				scroll := core.Style{Width: core.Pt(200), Height: core.Pt(200)}
				if horizontal {
					scroll.Direction = core.Row
				}
				return core.View{TestID: "sticky-root", Style: core.Style{FlexGrow: 1, Background: core.RGB(0xffffff)}, Children: []core.Node{
					core.Scroll{Ref: ref, TestID: "sticky-scroll", Horizontal: horizontal, HideIndicator: true, Style: scroll, Children: []core.Node{
						core.View{StyleFunc: func() core.Style {
							s := core.Style{Width: core.Pt(200), Height: core.Pt(length.Get()), Background: core.RGB(0x3366cc)}
							if horizontal {
								s.Width, s.Height = core.Pt(length.Get()), core.Pt(200)
							}
							return s
						}},
						core.View{TestID: "sticky-footer", StyleFunc: func() core.Style {
							s := core.Style{Width: core.Pt(200), Height: core.Pt(40), Position: core.Sticky, Bottom: core.Pt(inset.Get()), Background: core.RGB(0xff0000)}
							if horizontal {
								s.Width, s.Height, s.Right = core.Pt(40), core.Pt(200), core.Pt(inset.Get())
							}
							if !pinned.Get() {
								s.Position, s.Bottom, s.Right = core.Relative, core.Length{}, core.Length{}
							}
							return s
						}},
					}},
				}}
			}, nsgotest.Size(240, 240))
			check := func(want core.Points) {
				t.Helper()
				app.Settle()
				scroll, footer := app.Find("id:sticky-scroll"), app.Find("id:sticky-footer")
				got := footer.Frame.Origin.Y - scroll.Offset.Y + scroll.StickyShift(footer.ID)
				if horizontal {
					got = footer.Frame.Origin.X - scroll.Offset.X + scroll.StickyShift(footer.ID)
				}
				if math.Abs(float64(got-want)) > 0.5 {
					t.Fatalf("footer at %v, want %v (offset %+v)", got, want, scroll.Offset)
				}
				if nsgotest.Target() == "" {
					return
				}
				img := app.Capture()
				root := app.Find("id:sticky-root")
				for root.Parent != nil {
					root = root.Parent
				}
				for _, sample := range []struct {
					at  core.Points
					red bool
				}{
					{want - 4, false}, {want + 4, true}, {want + 36, true}, {want + 44, false},
				} {
					x, y := core.Points(20), sample.at
					if horizontal {
						x, y = sample.at, 20
					}
					px := img.Bounds().Min.X + int(float64(x)*float64(img.Bounds().Dx())/float64(root.Frame.Size.Width))
					py := img.Bounds().Min.Y + int(float64(y)*float64(img.Bounds().Dy())/float64(root.Frame.Size.Height))
					r, g, b, _ := img.At(px, py).RGBA()
					red := r >= 50000 && g <= 18000 && b <= 18000
					if red != sample.red {
						t.Fatalf("native footer pixel at (%v,%v) = (%d,%d,%d), red = %v, want %v", x, y, r, g, b, red, sample.red)
					}
				}
			}
			change := func(fn func()) { scope.Post(fn); app.Settle() }
			check(160)
			change(func() {
				at := core.Point{Y: 250}
				if horizontal {
					at = core.Point{X: 250}
				}
				ref.ScrollTo(at, false)
			})
			check(160)
			change(func() { inset.Set(10) })
			check(150)
			change(func() {
				at := core.Point{Y: 440}
				if horizontal {
					at = core.Point{X: 440}
				}
				ref.ScrollTo(at, false)
				pinned.Set(false)
			})
			check(160)
			change(func() { pinned.Set(true) })
			check(150)
			change(func() { ref.ScrollTo(core.Point{}, false); length.Set(60) })
			check(60)
		})
	}
}
