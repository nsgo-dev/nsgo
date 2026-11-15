package textview

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/nsgo-dev/nsgo/core"
)

// maxMatches caps the matches a view keeps, and finds in each of its texts, so an untrusted document cannot allocate without bound.
const maxMatches = 1000

// FindOptions say how a search matches: by default case-insensitively, with Unicode's simple case folding, and anywhere in a word.
type FindOptions struct {
	CaseSensitive bool
	// WholeWord matches only where no letter, digit, mark or underscore adjoins a match's ends that are word characters themselves.
	WholeWord bool
}

// TextRef names a text a view shows.
type TextRef struct {
	// Footnote is the footnote's index in the document's Footnotes, -1 outside footnotes.
	Footnote int
	// Path is the text's place in the blocks, outermost first: a block's index among its siblings; inside a list the item's index, then the block's in it; inside a table the row's (0 the head, the body's rows from 1), then the cell's; inside a details -1 for its summary, else a child block's index.
	Path []int
}

// child is the place of the i'th block or part inside the block at ref.
func (ref TextRef) child(i ...int) TextRef {
	path := make([]int, len(ref.Path), len(ref.Path)+len(i))
	copy(path, ref.Path)
	return TextRef{Footnote: ref.Footnote, Path: append(path, i...)}
}

// key orders texts as their places do, a footnote's after the blocks, as a string compared bytewise: each number is eight hexadecimal digits, one more than itself so that -1 comes first.
func (ref TextRef) key() string {
	const hex = "0123456789abcdef"
	b := make([]byte, 0, 8*(len(ref.Path)+1))
	put := func(n int) {
		u := uint32(n + 1)
		for shift := 28; shift >= 0; shift -= 4 {
			b = append(b, hex[u>>uint(shift)&0xf])
		}
	}
	put(ref.Footnote)
	for _, n := range ref.Path {
		put(n)
	}
	return string(b)
}

// Match is a match in a text a view shows, From to To in UTF-16 offsets of that text, as core's text ranges take them.
type Match struct {
	Text     TextRef
	From, To int
}

// textRange is a match in one text, in UTF-16 offsets.
type textRange struct{ from, to int }

// finder finds one query in texts: KMP over its characters, folded unless the search is case-sensitive.
type finder struct {
	query       []rune
	fail        []int
	opts        FindOptions
	first, last bool // whether the query starts and ends with word characters
}

// newFinder returns the finder of query, nil for an empty one, which matches nothing.
func newFinder(query string, opts FindOptions) *finder {
	if query == "" {
		return nil
	}
	f := &finder{opts: opts}
	for _, c := range query {
		f.query = append(f.query, f.fold(c))
	}
	f.first, f.last = isWord(f.query[0]), isWord(f.query[len(f.query)-1])
	f.fail = make([]int, len(f.query))
	for i, k := 1, 0; i < len(f.query); i++ {
		for k > 0 && f.query[i] != f.query[k] {
			k = f.fail[k-1]
		}
		if f.query[i] == f.query[k] {
			k++
		}
		f.fail[i] = k
	}
	return f
}

// fold is c as the finder compares it: the least character of its simple case folding's orbit unless the search is case-sensitive.
func (f *finder) fold(c rune) rune {
	if f.opts.CaseSensitive {
		return c
	}
	least := c
	for o := unicode.SimpleFold(c); o != c; o = unicode.SimpleFold(o) {
		least = min(least, o)
	}
	return least
}

func isWord(c rune) bool {
	return c == '_' || unicode.IsLetter(c) || unicode.IsNumber(c) || unicode.IsMark(c)
}

// find returns up to limit matches of the query in text, apart and in order, in UTF-16 offsets.
func (f *finder) find(text string, limit int) []textRange {
	if f == nil || limit <= 0 {
		return nil
	}
	var out []textRange
	runes := []rune(text)
	// units[i] is the UTF-16 offset of runes[i].
	units := make([]int, len(runes)+1)
	for i, c := range runes {
		units[i+1] = units[i] + runeUnits(c)
	}
	m := len(f.query)
	for i, k := 0, 0; i < len(runes); i++ {
		c := f.fold(runes[i])
		for k > 0 && c != f.query[k] {
			k = f.fail[k-1]
		}
		if c == f.query[k] {
			k++
		}
		if k < m {
			continue
		}
		start, end := i+1-m, i+1
		if !f.opts.WholeWord || f.wholeWord(runes, start, end) {
			out = append(out, textRange{units[start], units[end]})
			if len(out) == limit {
				return out
			}
			k = 0
			continue
		}
		k = f.fail[k-1]
	}
	return out
}

// wholeWord reports whether no word character adjoins the match from start to end where the query's own end is one.
func (f *finder) wholeWord(runes []rune, start, end int) bool {
	return (!f.first || start == 0 || !isWord(runes[start-1])) && (!f.last || end == len(runes) || !isWord(runes[end]))
}

// spansText is the text spans make, as core lays it out: an attachment is one U+FFFC.
func spansText(spans []core.Span) string {
	var b strings.Builder
	for _, sp := range spans {
		if sp.Attachment != nil {
			b.WriteRune('\ufffc')
			continue
		}
		b.WriteString(sp.Content)
	}
	return b.String()
}

// unitLen is s's length in UTF-16 units.
func unitLen(s string) int {
	n := 0
	for _, c := range s {
		n += runeUnits(c)
	}
	return n
}

// runeUnits is c's length in UTF-16 units; one for a character UTF-16 cannot hold, which core writes as U+FFFD.
func runeUnits(c rune) int { return max(utf16.RuneLen(c), 1) }

// byteAt is the byte offset of UTF-16 offset u in s, on a character's boundary.
func byteAt(s string, u int) int {
	at := 0
	for i, c := range s {
		if at >= u {
			return i
		}
		at += runeUnits(c)
	}
	return len(s)
}

// searchSpans splits spans at the ranges, UTF-16 offsets in their text, in order and apart: each piece inside one takes PartSearch, and PartSearchSelected after it in the range at selected.
func (r *renderer) searchSpans(spans []core.Span, ranges []textRange, selected int) []core.Span {
	// The spans are always new: a text's SpansFunc merges their StyleFunc into them.
	if len(ranges) == 0 {
		return slices.Clone(spans)
	}
	out := make([]core.Span, 0, len(spans)+2*len(ranges))
	at, k := 0, 0
	for _, sp := range spans {
		if sp.Attachment != nil {
			out = append(out, sp)
			at++
			continue
		}
		n := unitLen(sp.Content)
		end := at + n
		cut := 0 // how far into the span the pieces reach, in units
		for ; k < len(ranges) && ranges[k].from < end; k++ {
			from, to := max(ranges[k].from-at, 0), min(ranges[k].to, end)-at
			if from > cut {
				out = append(out, piece(sp, cut, from))
			}
			if to > from {
				out = append(out, r.searchPiece(piece(sp, from, to), k == selected))
				cut = to
			}
			if ranges[k].to > end {
				break // it goes on in the next span
			}
		}
		if cut == 0 {
			out = append(out, sp)
		} else if cut < n {
			out = append(out, piece(sp, cut, n))
		}
		at = end
	}
	return out
}

// piece is sp's characters from UTF-16 offset from to to.
func piece(sp core.Span, from, to int) core.Span {
	s := sp.Content
	sp.Content = s[byteAt(s, from):byteAt(s, to)]
	return sp
}

// searchPiece is a piece of a match: the search's parts over the span's own.
func (r *renderer) searchPiece(sp core.Span, selected bool) core.Span {
	parts := []Part{PartSearch}
	if selected {
		parts = append(parts, PartSearchSelected)
	}
	sp.Class = joinClasses(sp.Class, r.class(parts...))
	own := sp.StyleFunc
	sp.StyleFunc = func() core.Style {
		var s core.Style
		if own != nil {
			s = own()
		}
		return s.Merge(r.style(parts...))
	}
	return sp
}

// pinned is spans with each attachment behind a pointer of its own, made once: core keeps the view of an attachment that a SpansFunc gives again as the same pointer, so a text restyles around its attachments as its matches change.
func pinned(spans []core.Span) []core.Span {
	out := slices.Clone(spans)
	for i, sp := range out {
		if sp.Attachment != nil {
			out[i].Attachment = &pinnedNode{sp.Attachment}
		}
	}
	return out
}

// pinnedNode is a node behind a pointer, which core tells apart by it.
type pinnedNode struct{ node core.Node }

// Render implements core.Node.
func (p *pinnedNode) Render(*core.Scope) core.Node { return p.node }

// resolvedSpans is spans with their ContentFunc read, since a text's SpansFunc reads Content alone.
func resolvedSpans(spans []core.Span) []core.Span {
	var out []core.Span
	for i, sp := range spans {
		if sp.ContentFunc == nil {
			continue
		}
		if out == nil {
			out = append([]core.Span(nil), spans...)
		}
		out[i].Content, out[i].ContentFunc = sp.ContentFunc(), nil
	}
	if out == nil {
		return spans
	}
	return out
}
