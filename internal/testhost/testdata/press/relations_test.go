package press_test

import (
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
	if got := app.Find("id:icon").Frame.Size; got.Width != 16 || got.Height != 16 {
		t.Fatalf("inherited icon size=%v", got)
	}
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
	app.Press("id:toggle")
	if got := app.Find("id:focus").Frame.Origin.X; got != 32 {
		t.Fatalf("removing icon left parent padding=%v", got)
	}
	app.Press("id:toggle")
	if got := app.Find("id:focus").Frame.Origin.X; got != 12 {
		t.Fatalf("adding icon did not restore parent padding=%v", got)
	}
}
