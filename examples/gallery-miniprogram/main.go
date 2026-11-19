// Command gallery-miniprogram is the gallery for mini programs: the components a mini program shows, in examples/gallery's look, small enough for the 2 MB package WeChat, QQ, Alipay and Douyin hold a mini program's module to, which examples/gallery is far past. An index of the demos by group pushes each demo over it, a page of cards under a header that goes back. Build it with go tool nsgo build examples/gallery-miniprogram, which builds the four mini programs, and add -target miniprogram_wechat -run to open WeChat's in its DevTools.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

func main() { nsgo.Run(Root, nsgo.WindowSize(390, 760)) }

// demo is an entry of the index: name is its ID and title, mark the letters on its tile, and screen its page's cards.
type demo struct {
	name, mark, detail string
	screen             func(cx *core.Scope, g *gallery) []core.Node
}

// group is demos by what they show.
type group struct {
	name  string
	demos []demo
}

// groups are the demos, grouped as examples/gallery groups its own.
var groups = []group{
	{"Actions", []demo{
		{"Button", "Bt", "Variants, sizes and a counter", buttonDemo},
	}},
	{"Inputs", []demo{
		{"Input", "In", "Text fields and what they hold", inputDemo},
		{"Toggles", "Tg", "Switches, checkboxes and radios", togglesDemo},
		{"Slider", "Sl", "A value dragged along a track", sliderDemo},
	}},
	{"Display", []demo{
		{"Badge", "Bd", "Badges, avatars and progress", badgeDemo},
		{"Image", "Im", "Pictures from the package and the web", imageDemo},
	}},
	{"Data", []demo{
		{"List", "Li", "A long list that selects a row", listDemo},
	}},
	{"Layout", []demo{
		{"Layout", "Ly", "Rows, growing and wrapping", layoutDemo},
	}},
	{"Navigation", []demo{
		{"Tabs", "Tb", "Tabs that switch a view", tabsDemo},
		{"NavStack", "Nv", "Pages pushed and popped", navigationDemo},
	}},
	{"Platform", []demo{
		{"Services", "Sv", "Alerts, the clipboard and storage", servicesDemo},
	}},
}

// findDemo returns the demo named name.
func findDemo(name string) (demo, bool) {
	for _, g := range groups {
		for _, d := range g.demos {
			if d.name == name {
				return d, true
			}
		}
	}
	return demo{}, false
}

// route is a page of the stack: the index (no demo), a demo, or a page the NavStack demo pushed, depth deep.
type route struct {
	demo  string
	depth int
}

// gallery is what the pages share: the stack they are on.
type gallery struct {
	nav *base.Nav[route]
}

// Root is the gallery: the index, with the demos pushed over it.
func Root(cx *core.Scope) core.Node {
	g := &gallery{nav: base.NewNav(cx, route{})}
	c := colorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return fillPage.Merge(core.Style{Background: c.background(), Color: c.text()})
	}, Children: []core.Node{
		g.nav.Stack(fillPage, func(cx *core.Scope, r route, _ func() int) base.NavPage {
			switch {
			case r.depth > 0:
				return g.page(cx, "Page "+itoa(r.depth), "page-"+itoa(r.depth), cards(pushedPage(cx, g, r.depth)))
			case r.demo != "":
				d, _ := findDemo(r.demo)
				return g.page(cx, d.name, "screen-"+d.name, cards(d.screen(cx, g)))
			}
			// The index is under the platform's navigation bar, which names the app.
			p := g.page(cx, "Gallery", "index", g.index(cx))
			p.Header = nil
			return p
		}),
	}}
}

// page is a page of the stack under its header.
func (g *gallery) page(cx *core.Scope, title, id string, content core.Node) base.NavPage {
	c := colorsOf(cx)
	return base.NavPage{Title: title, TestID: id, Header: g.header(cx, title),
		HeaderStyleFunc: func() core.Style {
			return core.Style{Background: c.background(), Color: c.text(), TintColor: c.accent()}
		},
		StyleFunc: func() core.Style { return core.Style{Background: c.background()} },
		Children:  []core.Node{content}}
}

// header is a page's header where the app draws it, as mini programs have it: back while the page is above another, and the title.
func (g *gallery) header(cx *core.Scope, title string) core.Node {
	c := colorsOf(cx)
	heading := text(title, 15, semibold, c.text)
	heading.MaxLines, heading.Role, heading.Style.FlexGrow, heading.Style.FlexShrink = 1, core.RoleHeader, 1, 1
	return core.View{Style: core.Style{Height: core.Pt(44), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1}},
		StyleFunc: func() core.Style {
			return core.Style{Background: c.background(), BorderColors: core.EdgeColors{Bottom: c.line()}}
		},
		Children: []core.Node{
			core.Show{When: func() bool { return g.nav.Depth() > 0 }, Then: func(*core.Scope) core.Node {
				return core.View{OnPress: func(core.PressEvent) { g.nav.Pop(1) }, Role: core.RoleButton, Label: "Back", TestID: "back", HitSlop: core.All(6),
					Style:     core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.Edges{Left: 4, Right: 10}, CornerRadius: radiusControl},
					StyleFunc: func() core.Style { return core.Style{Active: &core.Style{Background: c.pressed()}} },
					Children:  []core.Node{text("‹", 22, regular, c.accent), text("Back", 13, medium, c.accent)}}
			}},
			heading,
		}}
}

// index is the demos by group, each a card of rows, as the gallery's phone layout lists them.
func (g *gallery) index(cx *core.Scope) core.Node {
	c := colorsOf(cx)
	var nodes []core.Node
	for _, grp := range groups {
		var rows []core.Node
		for i, d := range grp.demos {
			rows = append(rows, demoRow(cx, d, i == 0, func() { g.nav.Push(route{demo: d.name}) }))
		}
		nodes = append(nodes, core.View{TestID: "group-" + grp.name, Style: core.Style{Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Padding: core.Symmetric(0, 4)}, Children: []core.Node{
				core.Text{Content: grp.name, Role: core.RoleHeader, Style: core.Style{FlexGrow: 1, FontSize: 13, FontWeight: semibold,
					LetterSpacing: 0.4, TextTransform: core.TextTransformUppercase}, StyleFunc: func() core.Style { return core.Style{Color: c.muted()} }},
				core.Text{Content: itoa(len(grp.demos)), Style: core.Style{FontSize: 12, FontFamily: core.FontMonospace}, StyleFunc: func() core.Style { return core.Style{Color: c.faint()} }},
			}},
			core.View{Style: core.Style{CornerRadius: radiusCard, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Overflow: core.Hidden},
				StyleFunc: func() core.Style { return core.Style{Background: c.surface(), BorderColor: c.border()} }, Children: rows},
		}})
	}
	return core.Scroll{TestID: "home", Style: fillPage, Children: []core.Node{
		core.View{Style: core.Style{Gap: 20, Padding: core.All(16)}, Children: nodes},
	}}
}

// demoRow is a demo in the index: its tile, name and what it shows.
func demoRow(cx *core.Scope, d demo, first bool, open func()) core.Node {
	c := colorsOf(cx)
	s := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14, Padding: core.All(12)}
	if !first {
		s.Borders = core.Edges{Top: 1}
	}
	detail := text(d.detail, 13, regular, c.muted)
	detail.MaxLines = 2
	return core.View{OnPress: func(core.PressEvent) { open() }, Role: core.RoleButton, Label: d.name, TestID: "nav-" + d.name,
		StyleFunc: func() core.Style { return core.Style{Active: &core.Style{Background: c.pressed()}} },
		Children: []core.Node{core.View{Style: s, StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: c.line()}} },
			Children: []core.Node{
				tile(cx, d.mark, 38),
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{text(d.name, 16, medium, c.text), detail}},
				text("›", 20, regular, c.faint),
			}}}}
}

// tile is a rounded tile size points square around a mark, where the gallery shows the demo's icon.
func tile(cx *core.Scope, mark string, size core.Points) core.Node {
	c := colorsOf(cx)
	return core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size * 0.25, BorderCurve: core.BorderCurveContinuous,
		BorderWidth: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		StyleFunc: func() core.Style { return core.Style{BorderColor: c.border(), Background: c.raised()} },
		Children:  []core.Node{text(mark, size*0.36, semibold, c.accent)}}
}
