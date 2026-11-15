package base

import (
	"strconv"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// SuggestItem is a row of a SuggestList, which shows the rows in the app's order and filters none.
type SuggestItem struct {
	ID, Label, Detail string
	// Insert replaces the trigger and the query as the row is accepted; Label does when it is empty.
	Insert   string
	Disabled bool
}

// SuggestList shows the rows Items returns for a core.Suggest's query in an overlay at the caret, beside the field the Suggest follows, while the query is open and has rows, on the list Select and Combobox open. While it shows and no input method composes in the field, Up and Down move the highlight over the enabled rows and around the ends, Page Up and Page Down by a page, stopping at the ends, Return and Tab accept the highlighted row and Escape closes the query; the field keeps its own keys otherwise. A press accepts a row, on a desktop as the button goes down. The rows take no focus and a tap on them keeps the software keyboard up, so the field keeps the focus, and a highlight the keys move is announced while a screen reader runs. Give the field the combobox role, with Expanded following OnOpen and OnClose. It has no look.
type SuggestList struct {
	Suggest *core.Suggest
	// Items returns the rows for a query. It runs untracked on the engine goroutine, once per query while the query is open; an app may memoize it.
	Items func(query string) []SuggestItem

	// Placement, Align, Offset and Margin place the overlay beside the caret, as core.Overlay's.
	Placement      core.Placement
	Align          core.Align
	Offset, Margin core.Points

	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	ListClass      string
	ListStyle      core.Style
	ItemClass      string
	ItemStyle      core.Style
	// ItemStyleFunc is each row's style following what it reads: the highlighted row's Focused state among them.
	ItemStyleFunc func() core.Style
	// RenderItem draws row index of what Items returned, a text of its Label when nil.
	RenderItem func(item SuggestItem, index int) core.Node

	// OnOpen and OnClose hear the rows show and go.
	OnOpen, OnClose func()
	// Label names the rows' list for screen readers, apart from the field.
	Label  string
	TestID string
}

// Render implements core.Node.
func (l SuggestList) Render(cx *core.Scope) core.Node {
	s := l.Suggest
	rows := core.NewMemoFunc(cx, func() []SuggestItem {
		if !s.Open() || l.Items == nil {
			return nil
		}
		q := s.Query()
		return core.Untracked(cx, func() []SuggestItem { return l.Items(q) })
	}, nil)
	// The rows are valued by their index: the app's IDs need not be unique, and the highlight starts on the first enabled row as the query changes.
	items := func() []SelectItem {
		r := rows.Get()
		out := make([]SelectItem, len(r))
		for i, it := range r {
			out[i] = SelectItem{Value: strconv.Itoa(i), Title: it.Label, Disabled: it.Disabled}
		}
		return out
	}
	none := ""
	lb := newListbox(cx, items, func() bool { return true }, listQuery{controlled: func() *string { return &none }})
	shown := func() bool { return s.Open() && len(rows.Get()) > 0 }
	first := func(shown *listShown, _ []SelectItem) int { return shown.step(-1, listMove{kind: moveFirst}) }
	was := false
	cx.Effect(func() {
		now := shown()
		rows.Get()
		cx.Untrack(func() {
			switch {
			case now && !was:
				lb.opened(first)
				if l.OnOpen != nil {
					l.OnOpen()
				}
			case now:
				lb.highlightFirst()
			case was:
				lb.closed()
				if l.OnClose != nil {
					l.OnClose()
				}
			}
			was = now
		})
	})
	accept := func(item int) {
		r := rows.GetUntracked()
		if item < 0 || item >= len(r) || r[item].Disabled {
			return
		}
		text := r[item].Insert
		if text == "" {
			text = r[item].Label
		}
		s.Accept(text)
	}
	// The keys are bound while the rows show, beside the field that edits, as a search field's: an input method composing keeps them.
	keys := core.Show{When: func() bool { return shown() && !s.Composing() }, Then: func(cx *core.Scope) core.Node {
		move := func(key string, by func() listMove) {
			cx.OnKeyWith(key, core.KeyOptions{When: core.KeyAlways, Repeat: true}, func() { lb.moveBy(by()) })
		}
		fixed := func(kind moveKind) func() listMove { return func() listMove { return listMove{kind: kind} } }
		move("up", fixed(movePrevious))
		move("down", fixed(moveNext))
		move("pageup", func() listMove { return listMove{movePageUp, lb.page()} })
		move("pagedown", func() listMove { return listMove{movePageDown, lb.page()} })
		cx.OnKeyWith("escape", core.KeyOptions{When: core.KeyAlways}, s.Close)
		// With no enabled row to accept, Return and Tab stay the field's.
		return core.Show{When: func() bool { return lb.highlight.Get() >= 0 }, Then: func(cx *core.Scope) core.Node {
			for _, key := range []string{"enter", "tab"} {
				cx.OnKeyWith(key, core.KeyOptions{When: core.KeyAlways}, func() { accept(lb.highlight.GetUntracked()) })
			}
			return nil
		}}
	}}
	list := listParts{list: part{Class: l.ListClass, Style: l.ListStyle}, item: parts.Of(l.ItemClass, l.ItemStyle, l.ItemStyleFunc),
		listLabel: l.Label, keyboardTaps: core.KeyboardTapsHandled}
	if l.RenderItem != nil {
		list.renderItem = func(e SelectEntry) core.Node {
			if r := rows.Get(); e.Index < len(r) {
				return l.RenderItem(r[e.Index], e.Index)
			}
			return nil
		}
	}
	hooks := listHooks{open: shown, choose: accept, selected: func(string) bool { return false }}
	return core.Group{keys, core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
		leaveBar(cx)
		return core.Overlay{AnchorBounds: s.Anchor, Placement: l.Placement, Align: l.Align, Offset: l.Offset, Margin: l.Margin, HoldToRoom: true,
			Class: l.PopupClass, Style: l.PopupStyle, StyleFunc: l.PopupStyleFunc, TestID: l.TestID, Children: lb.content(false, list, hooks)}
	}}}
}
