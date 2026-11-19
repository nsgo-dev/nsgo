package main

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// The gallery on an Apple Watch: a list of the demos a watch shows, each pushing a page of its own laid out for a screen about 200 points wide, with a back button in its header. The other demos' pages are laid out for phones and wider, and many need what watchOS has not: web views, video, shaders, native components, pickers, the clipboard or a keyboard.

// watchPages are the demos a watch shows, by name, each a page of its own.
var watchPages = map[string]func(cx *core.Scope) core.Node{
	"Button": watchButtons, "Checkbox": watchToggles, "Slider": watchSlider, "Badge": watchBadges, "Avatar": watchAvatars,
	"Progress": watchProgress, "Toast": watchToast, "Dialog": watchDialog, "Text": watchText,
}

// watchDemos are the demos a watch shows, in the index's order.
func watchDemos() []demo {
	var out []demo
	for _, d := range allDemos() {
		if watchPages[d.name] != nil {
			out = append(out, d)
		}
	}
	return out
}

// watch is the watch's layout: the index at the bottom of the stack, the demo pushed over it. None of the watch's pages pushes another.
func (g *gallery) watch(cx *core.Scope) core.Node {
	return g.nav.Stack(fillPage, func(cx *core.Scope, r route, _ func() int) base.NavPage {
		tr := locales.For(cx)
		if r.demo != "" {
			d, _ := findDemo(r.demo)
			return watchScreen(cx, d.title(tr), "screen-"+d.name, true, watchPages[d.name](cx))
		}
		return watchScreen(cx, tr.AppGallery(), "index", false, g.watchIndex(cx))
	})
}

// watchScreen is a page of the watch's stack under its own header, with a back button above the index: the system draws none on a watch. The content, which watchScroll pads below the header, fills the screen under it, as the host's scroll views start scrolled back by as much as they are below the screen's top.
func watchScreen(cx *core.Scope, title, testID string, back bool, content core.Node) base.NavPage {
	colors := ui.ColorsOf(cx)
	tr := locales.For(cx)
	heading := ui.Text(title, 15, ui.Semibold, colors.Text)
	heading.MaxLines, heading.Role, heading.Style.FlexShrink = 1, core.RoleHeader, 1
	header := []core.Node{heading}
	if back {
		header = []core.Node{core.View{OnPress: func(core.PressEvent) { shell.nav.Pop(1) }, Role: core.RoleButton, Label: tr.UiBack(), TestID: "back",
			HitSlop: core.All(8), Style: core.Style{Width: core.Pt(28), Height: core.Pt(28), CornerRadius: 14, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: ui.Filled(colors.Inset), Children: []core.Node{ui.Icon(lucide.ChevronLeft, 18, colors.Accent)}}, heading}
	}
	return base.NavPage{Title: title, TestID: testID, HeaderHidden: true, StyleFunc: pageStyle(cx), Children: []core.Node{
		core.View{Style: fillPage, Children: []core.Node{
			content,
			core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Height: core.Pt(watchHeader),
				Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Padding: core.Symmetric(0, 8)}, StyleFunc: pageStyle(cx), Children: header},
		}},
	}}
}

// watchIndex is the demos a watch shows, by group, each a row that pushes it.
func (g *gallery) watchIndex(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	var rows []core.Node
	for _, grp := range allGroups() {
		var demos []core.Node
		for _, d := range grp.demos {
			if watchPages[d.name] == nil {
				continue
			}
			title := ui.Text(d.title(tr), 15, ui.Medium, colors.Text)
			title.MaxLines, title.Style.FlexShrink = 1, 1
			demos = append(demos, core.View{OnPress: func(core.PressEvent) { g.nav.Push(route{demo: d.name}) }, Role: core.RoleButton, Label: d.title(tr), TestID: "nav-" + d.name,
				Style:     core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(40), Padding: core.Symmetric(6, 10), CornerRadius: 12},
				StyleFunc: ui.Filled(colors.Inset), Children: []core.Node{ui.Icon(d.icon, 18, colors.Accent), title}})
		}
		if len(demos) == 0 {
			continue
		}
		heading := core.Text{Content: grp.title(tr), Role: core.RoleHeader, Style: core.Style{FontSize: 12, FontWeight: ui.Semibold, Padding: core.Edges{Top: 6, Left: 4}}, StyleFunc: ui.Colored(colors.Muted)}
		rows = append(rows, heading)
		rows = append(rows, demos...)
	}
	return watchScroll("home", rows...)
}

// watchHeader is the height of a watch page's header.
const watchHeader core.Points = 32

// watchScroll scrolls a watch page's content under its header, a column on narrow gutters.
func watchScroll(testID string, children ...core.Node) core.Node {
	return core.Scroll{TestID: testID, Style: fillPage, Children: []core.Node{
		core.View{Style: core.Style{Gap: 6, Padding: core.Edges{Top: watchHeader + 4, Right: 8, Bottom: 24, Left: 8}}, Children: children},
	}}
}

// watchSection is a component's name above it, centered.
func watchSection(cx *core.Scope, name string, children ...core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	caption := core.Text{Content: name, Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)}
	return core.View{Style: core.Style{Gap: 6, Padding: core.Edges{Top: 6}}, Children: append([]core.Node{caption}, children...)}
}

// watchWrap lays nodes in rows that wrap, centered.
func watchWrap(nodes ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8}, Children: nodes}
}

func watchButtons(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	bold := core.NewSignal(cx, true)
	var buttons []core.Node
	for _, v := range buttonVariants {
		buttons = append(buttons, ui.Button{Text: v.Label, Variant: v.Value, OnPress: func() {}, Style: core.Style{Width: core.Pct(100)}})
	}
	var icons []core.Node
	for _, v := range []ui.IconButtonVariant{ui.IconButtonGhost, ui.IconButtonPrimary, ui.IconButtonRaised} {
		icons = append(icons, ui.IconButton{Icon: lucide.Star, Label: tr.LibStarred(), Variant: v, Size: 36, OnPress: func() {}})
	}
	return watchScroll("demo-scroll",
		watchSection(cx, "Button", buttons...),
		watchSection(cx, "IconButton", watchWrap(icons...)),
		watchSection(cx, "ToggleButton", watchWrap(ui.ToggleButton{Text: tr.LibBold(), Label: tr.LibBold(), On: bold.Get, OnChange: bold.Set})),
	)
}

func watchToggles(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	starred, on, picked := core.NewSignal(cx, true), core.NewSignal(cx, true), core.NewSignal(cx, 0)
	var radios []core.Node
	for i, name := range []string{tr.LibApple(), tr.LibBanana(), tr.LibCherry()} {
		radios = append(radios, ui.Radio{Text: name, Checked: func() bool { return picked.Get() == i }, OnChange: func(bool) { picked.Set(i) }})
	}
	return watchScroll("demo-scroll",
		watchSection(cx, "Checkbox", ui.Checkbox{Text: tr.LibStarred(), Checked: starred.Get, OnChange: starred.Set, TestID: "checkbox"}),
		watchSection(cx, "Switch", ui.HStack{Gap: 8, Children: []core.Node{
			ui.Switch{Label: tr.LibStarred(), Checked: on.Get, OnChange: on.Set, TestID: "switch"},
			ui.Text(tr.LibStarred(), 14, ui.Regular, ui.ColorsOf(cx).Text)}}),
		watchSection(cx, "RadioGroup", ui.RadioGroup{Label: tr.LibFruit(), Children: radios}),
	)
}

func watchSlider(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	volume, stars := core.NewSignal(cx, 30.0), core.NewSignal(cx, 3)
	return watchScroll("demo-scroll",
		watchSection(cx, "Slider", ui.Slider{Label: tr.LibVolume(), Value: volume.Get, OnChange: volume.Set, TestID: "slider"},
			core.Text{ContentFunc: func() string { return itoa(int(volume.Get())) }, Style: core.Style{FontSize: 13, FontFamily: ui.Mono, TextAlign: core.TextAlignCenter},
				StyleFunc: ui.Colored(ui.ColorsOf(cx).Muted)}),
		watchSection(cx, "Rating", watchWrap(ui.Rating{Label: tr.KitRating(), Value: stars.Get, OnChange: stars.Set})),
	)
}

func watchBadges(cx *core.Scope) core.Node {
	var badges []core.Node
	for _, v := range badgeVariants {
		badges = append(badges, ui.Badge{Text: v.Label, Variant: v.Value})
	}
	return watchScroll("demo-scroll", watchSection(cx, "Badge", watchWrap(badges...)))
}

func watchAvatars(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	var members []ui.AvatarGroupMember
	for _, name := range demoPeople {
		members = append(members, ui.AvatarGroupMember{Name: name})
	}
	return watchScroll("demo-scroll",
		watchSection(cx, "Avatar", watchWrap(ui.Avatar{Name: demoPeople[0], Size: 40, Status: ui.AvatarOnline}, ui.Avatar{Name: demoPeople[1], Size: 40, Image: core.ImageSource{Data: logoPNG}})),
		watchSection(cx, "AvatarGroup", watchWrap(ui.AvatarGroup{Members: members, Max: 4, Size: 30, Label: tr.ControlsMembers(), MoreLabel: func(n int) string { return "+" + itoa(n) }})),
	)
}

func watchProgress(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	return watchScroll("demo-scroll",
		watchSection(cx, "Progress", ui.Progress{Label: tr.LibProgress(), Value: func() float32 { return 40 }}),
		watchSection(cx, "ProgressRing", watchWrap(ui.ProgressRing{Label: tr.LibProgress(), Size: 40, Value: func() float32 { return 60 }},
			ui.ProgressRing{Label: tr.LibProgress(), Size: 40, Indeterminate: func() bool { return true }})),
		watchSection(cx, "Spinner", watchWrap(ui.Spinner{Label: tr.LibLoading(), Variant: ui.SpinnerRing, Size: 24}, ui.Spinner{Label: tr.LibLoading(), Variant: ui.SpinnerSpokes, Size: 24})),
		watchSection(cx, "Meter", ui.Meter{Value: func() float64 { return 0.6 }, Variant: ui.MeterAccent, Marker: 0.8}),
	)
}

func watchToast(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	return watchScroll("demo-scroll", watchSection(cx, "Toasts",
		ui.Button{Text: tr.KitShow(), TestID: "toast-show", Style: core.Style{Width: core.Pct(100)}, OnPress: func() {
			shell.toasts.Show(ui.ToastContent{Title: tr.LibChangesSaved()})
		}}))
}

func watchDialog(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	answer := core.NewSignal(cx, "")
	return watchScroll("demo-scroll", watchSection(cx, "ShowAlert",
		ui.Button{Text: tr.KitOpen(), TestID: "alert-open", Style: core.Style{Width: core.Pct(100)}, OnPress: func() {
			actions := []core.AlertAction{{Label: tr.KitCancel(), Role: core.AlertCancel}, {Label: tr.OverlayDone()}}
			cx.ShowAlert(core.AlertConfig{Title: tr.KitDialogTitle(), Message: tr.KitDialogNote(), Actions: actions}, func(i int) {
				if i >= 0 {
					answer.Set(actions[i].Label)
				}
			})
		}},
		core.Text{TestID: "alert-answer", ContentFunc: answer.Get, Style: core.Style{FontSize: 13, TextAlign: core.TextAlignCenter}, StyleFunc: ui.Colored(ui.ColorsOf(cx).Muted)},
	))
}

func watchText(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tr := locales.For(cx)
	line := func(size core.Points, weight core.FontWeight, family string) core.Node {
		t := ui.Text(tr.DemoTextDetail(), size, weight, colors.Text)
		t.Style.FontFamily = family
		return t
	}
	return watchScroll("demo-scroll",
		watchSection(cx, "Title", line(20, ui.Bold, "")),
		watchSection(cx, "Body", line(15, ui.Regular, "")),
		watchSection(cx, "Mono", line(12, ui.Regular, ui.Mono)),
		watchSection(cx, "MaxLines = 1", core.Text{Content: tr.DemoTextDetail(), MaxLines: 1, Style: core.Style{FontSize: 15}, StyleFunc: ui.Colored(colors.Muted)}),
	)
}
