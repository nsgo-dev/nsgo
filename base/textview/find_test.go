package textview

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/core"
)

func TestFindMatchesCaseAndWholeWords(t *testing.T) {
	for _, c := range []struct {
		text, query string
		opts        FindOptions
		want        []textRange
	}{
		{"README readme ReadMe", "readme", FindOptions{}, []textRange{{0, 6}, {7, 13}, {14, 20}}},
		{"README readme ReadMe", "readme", FindOptions{CaseSensitive: true}, []textRange{{7, 13}}},
		// The Kelvin sign folds to k as K does.
		{"Kelvin", "kelvin", FindOptions{}, []textRange{{0, 6}}},
		{"read readme read_x (read).", "read", FindOptions{WholeWord: true}, []textRange{{0, 4}, {20, 24}}},
		// A query that ends in no word character needs no boundary there.
		{"a.b a.c", "a.", FindOptions{WholeWord: true}, []textRange{{0, 2}, {4, 6}}},
		// Matches are apart, and found again after a false start.
		{"aaaa aab", "aa", FindOptions{}, []textRange{{0, 2}, {2, 4}, {5, 7}}},
		{"aaab", "aab", FindOptions{}, []textRange{{1, 4}}},
		// UTF-16 offsets: an emoji outside the BMP is two units.
		{"😀 readme", "readme", FindOptions{}, []textRange{{3, 9}}},
		{"日本語", "本", FindOptions{}, []textRange{{1, 2}}},
		{"anything", "", FindOptions{}, nil},
	} {
		if got := newFinder(c.query, c.opts).find(c.text, maxMatches); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q in %q (%+v): %v, want %v", c.query, c.text, c.opts, got, c.want)
		}
	}
}

func TestFindStopsAtTheCap(t *testing.T) {
	if n := len(newFinder("a", FindOptions{}).find(strings.Repeat("a ", 5000), maxMatches)); n != maxMatches {
		t.Errorf("%d matches, want the cap %d", n, maxMatches)
	}
}

func TestPlacesOrderTheirTexts(t *testing.T) {
	main := TextRef{Footnote: -1}
	refs := []TextRef{main.child(0), main.child(1, -1), main.child(1, 0), main.child(1, 0, 2), main.child(1, 1), main.child(12), {Footnote: 0, Path: []int{0}}, {Footnote: 1, Path: []int{0}}}
	for i := 1; i < len(refs); i++ {
		if refs[i-1].key() >= refs[i].key() {
			t.Errorf("%v does not come before %v", refs[i-1], refs[i])
		}
	}
	// A child's path is its own, not its parent's.
	parent := main.child(3)
	a, b := parent.child(0), parent.child(1)
	if a.Path[1] != 0 || b.Path[1] != 1 || len(parent.Path) != 1 {
		t.Errorf("children %v and %v of %v", a, b, parent)
	}
}

// Splitting cuts spans on characters, at UTF-16 offsets of their text, past attachments, keeping each span's own fields.
func TestSearchSpansSplitAtMatches(t *testing.T) {
	r := &renderer{tv: TextView{Classes: Classes{PartSearch: "hit", PartSearchSelected: "selected"}}}
	press := func(core.PressEvent) {}
	spans := []core.Span{{Content: "a"}, {Content: "[1]", OnPress: press, OnLongPress: press}, {Attachment: core.View{}}, {Content: "日本"}, {Content: "語 readme", Class: "code"}}
	text := spansText(spans)
	ranges := newFinder("[1]", FindOptions{}).find(text, maxMatches)
	ranges = append(ranges, newFinder("本語 rea", FindOptions{}).find(text, maxMatches)...)
	got := r.searchSpans(spans, ranges, 1)
	var pieces []string
	for _, sp := range got {
		if sp.Attachment != nil {
			pieces = append(pieces, "@")
			continue
		}
		if !utf8.ValidString(sp.Content) {
			t.Errorf("piece %q is not UTF-8", sp.Content)
		}
		pieces = append(pieces, sp.Content+"/"+sp.Class)
	}
	want := []string{"a/", "[1]/hit", "@", "日/", "本/hit selected", "語 rea/code hit selected", "dme/code"}
	if !reflect.DeepEqual(pieces, want) {
		t.Errorf("pieces %q, want %q", pieces, want)
	}
	if got[1].OnPress == nil || got[1].OnLongPress == nil {
		t.Error("a match on a link lost its presses")
	}
	if s := got[5].StyleFunc; s == nil {
		t.Error("a match has no search style")
	}
}
