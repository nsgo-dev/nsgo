package main

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// The gallery on an Apple TV: a sidebar of the demos in type read from across a room, which the remote's focus moves through and its select button opens, beside the demo chosen under its title. A TV takes no touches and has no pointer, so the demos made of drags, pinches, hovers or text editing are left out, as are those of what tvOS has not: web views, the terminal's emulator and widgets.

// tvLeftOut are the demos a TV does not show.
var tvLeftOut = map[string]bool{
	// Touch: pull to refresh, paging, zoom and drags.
	"Photos": true, "Scroll": true, "Gestures": true, "Physics": true, "Collapsing header": true,
	// Drags: a split handle, a color area, resized and moved columns, panels and a camera.
	"Image": true, "ColorPicker": true, "DataTable": true, "Resizable": true, "Dock": true, "Camera": true, "FlowDiagram": true,
	// The pointer's hover.
	"Popover": true, "Plot": true,
	// Text edited with carets and selections.
	"Editor": true, "RichEditor": true,
	// What tvOS has not.
	"WebView": true, "Terminal": true, "Widgets": true,
}

// tvSidebarWidth is the TV sidebar's width.
const tvSidebarWidth core.Points = 460

// tv is the TV's layout: the sidebar, and the demo chosen beside it, with what it pushes over it.
func (g *gallery) tv(cx *core.Scope) core.Node {
	if tvLeftOut[g.selected.GetUntracked()] {
		g.selectDemo(tvDemos()[0].name)
	}
	// The demos lay themselves out in paneWidth, the window's width less the sidebar's.
	g.sidebarOpen.Set(true)
	g.sidebarWidth.Set(tvSidebarWidth)
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	return core.View{Style: fillPage.Merge(core.Style{Direction: core.Row}), Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(tvSidebarWidth), FlexShrink: 0, Borders: core.Edges{Right: 1}}, StyleFunc: func() core.Style {
			return core.Style{Background: colors.Surface(), BorderColors: core.EdgeColors{Right: colors.Line()}}
		}, Children: []core.Node{g.tvSidebar(cx)}},
		// The demo keeps clear of the screen's right edge, which a TV may not show.
		core.View{Style: fillPage, StyleFunc: func() core.Style {
			return core.Style{Padding: core.Edges{Right: env.SafeArea().Right}, Background: colors.Background()}
		}, Children: []core.Node{
			g.tvTitle(),
			g.nav.Stack(fillPage, func(cx *core.Scope, r route, _ func() int) base.NavPage {
				if r.page != nil {
					return r.page(cx)
				}
				return base.NavPage{HeaderHidden: true, Children: []core.Node{g.demoPane()}}
			}),
		}},
	}}
}

// tvDemos are the demos a TV shows, in the index's order.
func tvDemos() []demo {
	var out []demo
	for _, d := range allDemos() {
		if !tvLeftOut[d.name] {
			out = append(out, d)
		}
	}
	return out
}

// tvSidebar is every demo a TV shows, by group, the one shown selected.
func (g *gallery) tvSidebar(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	nodes := []core.Node{core.Text{Content: tr.AppGallery(), Role: core.RoleHeader, Style: core.Style{FontSize: 38, FontWeight: ui.Bold, Padding: core.Edges{Left: 16, Bottom: 8}}, StyleFunc: ui.Colored(colors.Text)}}
	for _, grp := range allGroups() {
		var rows []core.Node
		for _, d := range grp.demos {
			if tvLeftOut[d.name] {
				continue
			}
			rows = append(rows, core.View{OnPress: func(core.PressEvent) { g.selectDemo(d.name) }, Role: core.RoleButton, Label: d.title(tr), TestID: "nav-" + d.name,
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16, Height: core.Pt(64), Padding: core.Symmetric(0, 16), CornerRadius: 14},
				StyleFunc: func() core.Style {
					s := core.Style{Color: colors.Text(), Focus: &core.Style{Background: colors.Accent(), Color: colors.OnAccent(), Transform: core.Transform{Scale: 1.04}}}
					if g.selected.Get() == d.name {
						s.Background = colors.Pressed()
					}
					return s
				},
				Children: []core.Node{ui.Icon(d.icon, 28, ui.Inherited), core.Text{Content: d.title(tr), MaxLines: 1, Style: core.Style{FontSize: 28, FontWeight: ui.Medium, FlexShrink: 1}}}})
		}
		if len(rows) == 0 {
			continue
		}
		heading := core.Text{Content: grp.title(tr), Role: core.RoleHeader, Style: core.Style{FontSize: 22, FontWeight: ui.Semibold, Padding: core.Edges{Top: 24, Left: 16, Bottom: 4}}, StyleFunc: ui.Colored(colors.Muted)}
		nodes = append(nodes, heading, core.View{TestID: "nav-group-" + grp.name, Style: core.Style{Gap: 4}, Children: rows})
	}
	env := cx.Env()
	return core.Scroll{TestID: "sidebar", Style: fillPage, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			safe := env.SafeArea()
			return core.Style{Padding: core.Edges{Top: 60, Right: 24, Bottom: max(safe.Bottom, 24), Left: max(safe.Left-16, 24)}}
		}, Children: nodes},
	}}
}

// tvTitle is the demo shown's group above its name.
func (g *gallery) tvTitle() core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		name := g.selected.Get()
		d, _ := findDemo(name)
		return core.Func(func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			tr := locales.For(cx)
			title := ui.Text(d.title(tr), 48, ui.Bold, colors.Text)
			title.MaxLines, title.Role, title.TestID = 1, core.RoleHeader, "title"
			return core.View{Style: core.Style{Gap: 4, Padding: core.Edges{Top: 60, Right: 32, Bottom: 16, Left: 48}}, StyleFunc: pageStyle(cx), Children: []core.Node{
				ui.Text(groupOf(tr, name), 24, ui.Semibold, colors.Muted), title,
			}}
		})
	})
}
