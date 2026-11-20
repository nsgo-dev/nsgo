package chat

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/ui/chat/locales"
)

// findBar is a transcript's find bar, shown while its search is open: a field, the count, previous, next and close. inset is the room above it; bottom, when set, is the height of what covers the transcript's end, a floating composer, which a revealed match keeps clear of as it does of the bar.
func findBar(s *ConversationState, inset core.Points, bottom func() core.Points) core.Node {
	return core.Show{When: s.search.IsOpen, Then: func(cx *core.Scope) core.Node {
		search := s.search
		colors := tonesOf(cx)
		height := core.NewSignal(cx, core.Points(0))
		cx.Effect(func() {
			margin := core.Edges{Top: height.Get() + inset + 16}
			if bottom != nil {
				margin.Bottom = bottom() + 16
			}
			search.SetRevealMargin(margin)
		})
		// Always, or the field, which has the focus, would keep Escape.
		cx.OnKeyWith("escape", core.KeyOptions{When: core.KeyAlways}, s.closeSearch)
		count := func() string {
			n := search.Count()
			if n == 0 {
				return locales.NoResults(cx)
			}
			return locales.Matches(cx, search.Selected(), n)
		}
		return core.View{TestID: "chat-find", PointerEvents: core.PointerEventsBoxNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(inset + 8), Left: core.Pt(8), Right: core.Pt(8), AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Ref: s.bar, PointerEvents: core.PointerEventsAuto, Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), MaxWidth: core.Pt(520), Padding: core.Symmetric(4, 6), Gap: 4, AlignItems: core.AlignCenter, CornerRadius: 9}, StyleFunc: func() core.Style {
				return core.Style{Background: colors.Surface(), BorderWidth: 1, BorderColor: colors.Border(), Shadows: []core.BoxShadow{ui.Shadow(0, 3, 12, 0, ui.Black(0.16))}}
			}, OnLayout: func(frame core.Bounds) { height.Set(frame.Size.Height) }, Children: []core.Node{
				ui.Input{Value: search.Query, OnChange: search.SetQuery, Label: locales.Find(cx), Placeholder: locales.Find(cx), AutoFocus: true, TestID: "chat-find-input", Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(80), Height: core.Pt(28)}},
				core.Text{ContentFunc: count, Style: core.Style{MinWidth: core.Pt(78), TextAlign: core.TextAlignCenter, FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)},
				ui.IconButton{Icon: lucide.ChevronUp, LabelFunc: func() string { return locales.Previous(cx) }, OnPress: search.Previous, TestID: "chat-find-previous"},
				ui.IconButton{Icon: lucide.ChevronDown, LabelFunc: func() string { return locales.Next(cx) }, OnPress: search.Next, TestID: "chat-find-next"},
				ui.IconButton{Icon: lucide.X, LabelFunc: func() string { return locales.Close(cx) }, OnPress: s.closeSearch, TestID: "chat-find-close"},
			}},
		}}
	}}
}
