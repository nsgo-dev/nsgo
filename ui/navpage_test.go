package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// TestNavPage gives a page without a header of its own a themed one with a back button, below the status bar where the system has a header. On iOS and Android the system's header stands in for it, and on Android it is opaque in the surface's color, the page below it.
func TestNavPage(t *testing.T) {
	android := phoneEnv
	android.OS, android.HeaderHeight, android.SafeArea = core.OSAndroid, 56, core.Edges{Top: 47, Bottom: 34}
	for _, e := range []core.Environment{desktopEnv, android} {
		popped := false
		var nav *base.Nav[string]
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			nav = base.NewNav(cx, "home", "detail")
			return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
				return NavPage(cx, base.NavPage{Title: "Detail " + route, TestID: "page-" + route, Children: []core.Node{
					core.View{TestID: "content-" + route, Style: core.Style{FlexGrow: 1}},
				}}, NavHeader{TestID: "header", OnBack: func() { popped = true }, BackLabel: "Back",
					Trailing: []core.Node{IconButton{Icon: lucide.Plus, Label: "Add", OnPress: func() {}}}})
			})
		})
		if e.OS == core.OSAndroid {
			if app.Exists("page-detail >> header") {
				t.Fatal("Android: the page still draws NavHeader")
			}
			c := app.Find("id:page-detail").Screen
			if c.Title != "Detail detail" || c.HeaderHidden || c.HeaderTransparent || !c.CanGoBack || c.Right == 0 {
				t.Fatalf("Android: system header = %+v, want an opaque one titled Detail detail with a back button and the trailing views", c)
			}
			if c.Background != p.Surface || c.TitleColor != p.Text {
				t.Fatalf("Android: header colors = %x on %x, want the text on the surface", c.TitleColor, c.Background)
			}
			// The bar, and the status bar past the padding of start's root.
			if want := core.Points(56 + 47 - 16); c.HeaderHeight != want {
				t.Fatalf("Android: header height = %v, want %v", c.HeaderHeight, want)
			}
			if got := app.Find("content-detail").Frame.Origin.Y; got != c.HeaderHeight {
				t.Fatalf("Android: content top = %v, want below the header, at %v", got, c.HeaderHeight)
			}
			app.Find("Detail detail")
			app.HeaderBack("id:page-detail")
			if got := nav.Routes(); len(got) != 1 {
				t.Fatalf("Android: routes after the header's back = %v, want the detail popped", got)
			}
			continue
		}
		header := app.Find("page-detail >> header")
		if header.Style.Background != p.Surface {
			t.Fatalf("%v: header = %x, want the surface", e.OS, header.Style.Background)
		}
		app.Find("Detail detail")
		app.Press("role:button Back")
		if !popped {
			t.Fatalf("%v: the back button did nothing", e.OS)
		}
	}
}

// TestNavInset leaves room for the header only where the content runs under
// it, and the page's content starts under it there.
func TestNavInset(t *testing.T) {
	phone := phoneEnv
	phone.HeaderHeight, phone.SafeArea = 44, core.Edges{Top: 47, Bottom: 34}
	android := phone
	android.OS = core.OSAndroid
	for _, c := range []struct {
		env   core.Environment
		inset core.Points // past the status bar, when above 0
		top   core.Points
	}{{phone, 1, 0}, {android, 0, 44 + 47 - 16}, {desktopEnv, 0, 44}} {
		var inset core.Points
		app, _ := start(t, c.env, func(cx *core.Scope) core.Node {
			nav := base.NewNav(cx, "home")
			return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, _ string, _ func() int) base.NavPage {
				return NavPage(cx, base.NavPage{Title: "Home", Children: []core.Node{core.Func(func(cx *core.Scope) core.Node {
					return core.View{TestID: "content", Style: core.Style{FlexGrow: 1}, StyleFunc: func() core.Style {
						inset = NavInset(cx)
						return core.Style{}
					}}
				})}}, NavHeader{})
			})
		})
		if (inset > 47) != (c.inset > 0) {
			t.Fatalf("%v: inset = %v, want %v", c.env.OS, inset, c.inset)
		}
		if got := app.Find("content").Frame.Origin.Y; got != c.top {
			t.Fatalf("%v: content top = %v, want %v", c.env.OS, got, c.top)
		}
	}
}
