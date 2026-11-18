package press_test

import (
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestRelationalSelectorsFollowNativeInput(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		visible := core.NewSignal(cx, true)
		return core.View{Children: []core.Node{
			core.View{TestID: "toggle", Class: "h-8", OnPress: func(core.PressEvent) { visible.Set(!visible.Get()) }},
			core.View{TestID: "parent", Class: "p-8 has-[>svg]:p-3 [&_svg]:size-4 hover:[&_svg]:opacity-50 has-[:focus-visible]:opacity-25", Children: []core.Node{
				core.View{TestID: "focus", Class: "size-6", OnPress: func(core.PressEvent) {}},
				core.Show{When: visible.Get, Then: func(*core.Scope) core.Node {
					return core.Icon{TestID: "icon", SVG: core.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path d="M0 0H24V24H0Z"/></svg>`))}
				}},
			}},
		}}
	}, nsgotest.Size(300, 240), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	if got := app.Find("id:icon").Frame.Size; !near(got.Width, 16) || !near(got.Height, 16) {
		t.Fatalf("inherited icon size=%v", got)
	}
	// Touch targets have no pointer to hover with, and there only text fields take the keyboard focus, so hover and a pressable view's focus are checked headless and on desktops and the web.
	switch nsgotest.Target() {
	case "", "linux", "macos", "windows", "web":
		app.Hover("id:parent")
		if got := app.Find("id:icon").Style.Opacity; got != .5 {
			t.Fatalf("native ancestor hover opacity=%v", got)
		}
		app.HoverOut()
		if got := app.Find("id:icon").Style.Opacity; got != 1 {
			t.Fatalf("native hover exit opacity=%v", got)
		}
		app.Focus("id:focus")
		if got := app.Find("id:parent").Style.Opacity; got != .25 {
			t.Fatalf("descendant focus-visible opacity=%v", got)
		}
	}
	app.Press("id:toggle")
	if got := app.Find("id:focus").Frame.Origin.X; !near(got, 32) {
		t.Fatalf("removing icon left parent padding=%v", got)
	}
	app.Press("id:toggle")
	if got := app.Find("id:focus").Frame.Origin.X; !near(got, 12) {
		t.Fatalf("adding icon did not restore parent padding=%v", got)
	}
}

// near reports whether a frame value is want, give or take the pixel snapping of a screen whose scale is not a whole number, such as Android's 420 dpi (2.625 pixels a point), where 12 points are 31.5 pixels and show as 32.
func near(got core.Points, want float64) bool { return math.Abs(float64(got)-want) < .5 }
