package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// SuggestList is base.SuggestList in a menu's panel at the caret: its rows are a dropdown's, each a row's label over its detail, and the row the keys or the pointer are on is lit. Placement is the caret's side it goes on, below it by default.
type SuggestList struct {
	Suggest *core.Suggest
	Items   func(query string) []base.SuggestItem
	// Leading draws what goes before row index's label, such as an icon; nothing when nil.
	Leading   func(item base.SuggestItem, index int) core.Node
	Placement core.Placement
	// OnOpen and OnClose hear the rows show and go, for the field's Expanded.
	OnOpen, OnClose func()
	// Label names the rows' list for assistive technology, apart from the field.
	Label  string
	TestID string
}

// suggestWidth and suggestHeight bound a SuggestList's panel.
const (
	suggestWidth  core.Points = 280
	suggestHeight core.Points = 220
)

// Render implements core.Node.
func (l SuggestList) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	row, look := listRowStyle(cx, base.PresentPopup)
	popup, _ := pickerSurface(cx, base.PresentPopup)
	return base.SuggestList{Suggest: l.Suggest, Items: l.Items, Placement: l.Placement, Align: core.AlignStart, Offset: 4, Margin: 8,
		PopupStyle: core.Style{Width: core.Pt(suggestWidth), MaxHeight: core.Pt(suggestHeight)}, PopupStyleFunc: popup,
		ItemStyle: row, ItemStyleFunc: look, OnOpen: l.OnOpen, OnClose: l.OnClose, Label: l.Label, TestID: l.TestID,
		RenderItem: func(item base.SuggestItem, index int) core.Node {
			text := []core.Node{core.Text{Content: item.Label, MaxLines: 1, Style: TypeBody, StyleFunc: Colored(colors.Text)}}
			if item.Detail != "" {
				text = append(text, core.Text{Content: item.Detail, MaxLines: 1, Style: TypeCaption, StyleFunc: Colored(colors.Muted)})
			}
			children := []core.Node{}
			if l.Leading != nil {
				if lead := l.Leading(item, index); lead != nil {
					children = append(children, lead)
				}
			}
			children = append(children, core.View{Style: core.Style{MinWidth: core.Pt(0), FlexGrow: 1, FlexShrink: 1, Padding: core.Symmetric(4, 0)}, Children: text})
			return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexGrow: 1}, Children: children}
		}}
}
