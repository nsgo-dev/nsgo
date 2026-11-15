package core

import (
	"strings"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// SuggestOptions tune a Suggest.
type SuggestOptions struct {
	// Trigger starts a query, "@" when empty.
	Trigger string
	// AllowSpaces lets a query contain spaces.
	AllowSpaces bool
	// AllowedPrefixes are what may precede Trigger. When nil, a line start or a space is accepted.
	AllowedPrefixes []string
	// OnOpen and OnClose hear a query start and end.
	OnOpen, OnClose func()
}

// Suggest follows a trigger query in a text field, such as a mention after "@", and replaces it with the text a list beside the field accepts. It holds no rows: base.SuggestList shows them. Pass it to the field's Input.Suggest. It is created in the field's owner scope and is safe on that scope's engine goroutine, like signals and the rest of the reactive graph.
type Suggest struct {
	cx   *Scope
	opts SuggestOptions

	open   *Signal[bool]
	query  *Signal[string]
	anchor *Signal[Bounds]
	// focused and composing follow the field's keyboard focus and an input method's composition in it: a query is open only while the field has the focus, and a list takes no keys while something composes.
	focused, composing *Signal[bool]

	value     string
	selection TextRange
	from, to  int
	// dismissed is the offset of the trigger Escape, Close or Accept closed, or -1: the query stays closed on that trigger, moved by edits before it, while its match holds, as base/richeditor's Suggestion does.
	dismissed int
	// t and n are the field the Suggest is bound to.
	t *Tree
	n *node
}

// NewSuggest creates a suggestion state for a field in cx's scope.
func NewSuggest(cx *Scope, o SuggestOptions) *Suggest {
	if o.Trigger == "" {
		o.Trigger = "@"
	}
	return &Suggest{cx: cx, opts: o, open: NewSignal(cx, false), query: NewSignal(cx, ""), anchor: NewSignal(cx, Bounds{}),
		focused: NewSignal(cx, false), composing: NewSignal(cx, false), dismissed: -1}
}

// Open reports whether a query is active: the field has the focus, and its caret, with no selection, follows a trigger with nothing between them that ends the query.
func (s *Suggest) Open() bool { return s != nil && s.open.Get() }

// Query is the text after the trigger and before the caret.
func (s *Suggest) Query() string { return s.query.Get() }

// Anchor is the caret's latest page bounds, suitable for Overlay.AnchorBounds.
func (s *Suggest) Anchor() Bounds { return s.anchor.Get() }

// Composing reports whether an input method composes in the field, when a list leaves the keys to it.
func (s *Suggest) Composing() bool { return s != nil && s.composing.Get() }

// Accept replaces the trigger and the query with text, puts the caret after it and closes the query, dismissing the trigger as Close does, so a text that starts with the trigger, such as "@alice", does not reopen it. An uncontrolled field takes the text itself, a controlled one through its OnChange.
func (s *Suggest) Accept(text string) {
	if !s.Open() {
		return
	}
	start := UTF16Offset(s.value, s.from)
	end := UTF16Offset(s.value, s.to)
	if start < 0 || end < start || start > len(s.value) || end > len(s.value) {
		return
	}
	value := s.value[:start] + text + s.value[end:]
	caret := s.from + UTF16Len(text)
	s.value, s.selection = value, TextRange{Start: caret, End: caret}
	s.dismissed = s.from
	s.close()
	t, n := s.t, s.n
	if n == nil || n.deleted || n.input == nil {
		return
	}
	// An uncontrolled field takes the text itself; a controlled one gets it through OnChange, as the user's edits.
	if in := n.input; in.valueFn == nil {
		in.value = value
		t.markResolve(n)
	}
	if n.h.change != nil {
		n.h.change(value)
	}
	// After the value, which the commit sends before its commands.
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemSelection, ID: n.id, Selection: s.selection})
	t.changed = true
}

// Close ends an active query without changing the field. It stays closed on the trigger it closed, however the query grows or the caret moves within it, until that match ends or a trigger starts elsewhere.
func (s *Suggest) Close() {
	if s.open.GetUntracked() {
		s.dismissed = s.from
	}
	s.close()
}

func (s *Suggest) close() {
	if !s.open.GetUntracked() {
		return
	}
	s.open.Set(false)
	if s.opts.OnClose != nil {
		s.opts.OnClose()
	}
}

// bind attaches the Suggest to field n.
func (s *Suggest) bind(t *Tree, n *node) { s.t, s.n = t, n }

// changed follows the user's edit.
func (s *Suggest) changed(value string) {
	s.dismissed = mapOffset(s.dismissed, s.value, value)
	s.setValue(value)
	s.update()
}

// sync follows a value the app set, such as a draft it cleared on sending: the query the user typed ends with it, and the next edit or caret move starts another.
func (s *Suggest) sync(value string) {
	if value == s.value {
		return
	}
	s.dismissed = -1
	s.setValue(value)
	s.close()
}

// mapOffset follows UTF-16 offset at from text old to next, an edit between them: an offset before the edit stays, one after it moves by the edit's length, and one the edit replaced is -1, as is -1 itself.
func mapOffset(at int, old, next string) int {
	if at < 0 || old == next {
		return at
	}
	p := 0
	for p < len(old) && p < len(next) && old[p] == next[p] {
		p++
	}
	q := 0
	for q < len(old)-p && q < len(next)-p && old[len(old)-1-q] == next[len(next)-1-q] {
		q++
	}
	// Byte by byte the ends may split a rune; back them off to rune starts.
	for p > 0 && p < len(old) && !utf8.RuneStart(old[p]) {
		p--
	}
	for q > 0 && !utf8.RuneStart(old[len(old)-q]) {
		q--
	}
	start, end := UTF16Len(old[:p]), UTF16Len(old[:len(old)-q])
	switch {
	case at < start:
		return at
	case at >= end:
		return at + UTF16Len(next) - UTF16Len(old)
	}
	return -1
}

func (s *Suggest) setValue(value string) {
	s.value = value
	if n := UTF16Len(value); s.selection.End > n {
		s.selection = TextRange{Start: n, End: n}
	}
}

func (s *Suggest) selectionChanged(r TextRange) {
	s.selection = r
	s.update()
}

func (s *Suggest) caretMoved(e CaretEvent) { s.anchor.Set(e.PageBounds) }

// focus follows the field's keyboard focus; the list closes as it leaves.
func (s *Suggest) focus(on bool) {
	s.focused.Set(on)
	if !on {
		s.composing.Set(false)
		s.close()
	}
}

// compose follows an input method's composition in the field.
func (s *Suggest) compose(on bool) { s.composing.Set(on) }

func (s *Suggest) update() {
	if !s.focused.GetUntracked() || s.selection.Start != s.selection.End {
		s.close()
		return
	}
	from, to, query, ok := suggestionRange(s.value, s.selection.End, s.opts)
	if !ok {
		s.dismissed = -1
		s.close()
		return
	}
	if from == s.dismissed {
		s.close()
		return
	}
	s.dismissed = -1
	s.from, s.to = from, to
	s.query.Set(query)
	if !s.open.GetUntracked() {
		s.open.Set(true)
		if s.opts.OnOpen != nil {
			s.opts.OnOpen()
		}
	}
}

func suggestionRange(value string, caret int, o SuggestOptions) (from, to int, query string, ok bool) {
	if o.Trigger == "" {
		o.Trigger = "@"
	}
	if caret < 0 || caret > UTF16Len(value) {
		return
	}
	before := value[:UTF16Offset(value, caret)]
	at := strings.LastIndex(before, o.Trigger)
	if at < 0 {
		return
	}
	prefix := before[:at]
	allowed := false
	if o.AllowedPrefixes == nil {
		allowed = prefix == "" || strings.HasSuffix(prefix, " ") || strings.HasSuffix(prefix, "\n")
	} else {
		for _, p := range o.AllowedPrefixes {
			if strings.HasSuffix(prefix, p) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return
	}
	query = before[at+len(o.Trigger):]
	if !o.AllowSpaces && strings.ContainsAny(query, " \n\t") {
		query = ""
		return
	}
	return UTF16Len(before[:at]), caret, query, true
}
