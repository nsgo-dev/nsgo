//go:build darwin

package apple

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestOneLineMatchesTheFramesetter lays texts on one line out both ways,
// as a CTLine and through a framesetter: their sizes, pixels, lines and
// carets are the same.
func TestOneLineMatchesTheFramesetter(t *testing.T) {
	defer func() { oneLineFast = true }()
	texts := []string{"1810.HK", "Xiaomi 921", "+34.45", "-9.27%", "6.87M", "abc ", " lead", "Ünïcødé ﬁ", "שלום world", "日本語のテキスト", "😀 emoji", ""}
	for _, text := range texts {
		for _, align := range []driver.TextAlign{driver.TextAlignStart, driver.TextAlignCenter, driver.TextAlignEnd, driver.TextAlignJustify} {
			for _, rtl := range []bool{false, true} {
				for _, st := range []driver.TextStyle{
					{Font: driver.Font{Size: 13, Weight: 400}},
					{Font: driver.Font{Size: 17.5, Weight: 600, Italic: true}},
					{Font: driver.Font{Family: "Menlo", Size: 12, Weight: 400}, LetterSpacing: 0.5},
					{Font: driver.Font{Size: 15, Weight: 400, Variant: driver.FontTabularNums}},
				} {
					for _, lines := range []int{0, 1} {
						st.Color, st.Align, st.RTL = driver.RGB(0), align, rtl
						at := driver.AttributedText{Text: text, Style: st, MaxLines: lines}
						name := fmt.Sprintf("%q/%v/rtl=%v/%v/%d", text, align, rtl, st.Font, lines)
						compareOneLine(t, name, &at)
					}
				}
			}
		}
	}
}

// TestTextsMeasureWhatTheFramesetterDoes checks that a text measures the size CoreText's framesetter suggests, its width's fraction kept, on a line of its own or wrapped: the core rounds sizes up to a device pixel, and a whole point here would add up to a point to every text on a 2x screen.
func TestTextsMeasureWhatTheFramesetterDoes(t *testing.T) {
	defer func() { oneLineFast = true }()
	for _, fast := range []bool{false, true} {
		oneLineFast = fast
		for _, text := range []string{"abc", "Hello world", "-9.27%", "two\nlines", strings.Repeat("word ", 20)} {
			at := driver.AttributedText{Text: text, Style: driver.TextStyle{Font: driver.Font{Size: 13, Weight: 400}, Color: driver.RGB(0)}}
			p := TextEngine{}.Prepare(&at)
			for _, width := range []driver.Points{driver.Auto, 100} {
				w := float64(unbounded)
				if !width.IsAuto() {
					w = float64(width)
				}
				fs := ctFramesetterSuggestSize(p.(*prepared).framesetter(), 0, 0, 0, w, unbounded, 0)
				want := driver.Size{Width: driver.Points(fs.Width), Height: driver.Points(fs.Height)}
				if got := p.Measure(width); got != want {
					t.Errorf("fast=%v: %q at %v measures %v, want the framesetter's %v", fast, text, width, got, want)
				}
			}
			p.Release()
		}
	}
}

// TestALimitedTextIsAsTallAsItsLinesUncut checks that a text cut to its line limit, whose height the engine works out from the frame's lines, is as tall as the framesetter makes those lines uncut, and an empty text with a line height as tall as a line: a label must not change its height as its text grows past its limit or empties.
func TestALimitedTextIsAsTallAsItsLinesUncut(t *testing.T) {
	measure := func(text string, lines int, st driver.TextStyle) driver.Size {
		p := TextEngine{}.Prepare(&driver.AttributedText{Text: text, Style: st, MaxLines: lines})
		defer p.Release()
		return p.Measure(100)
	}
	for _, st := range []driver.TextStyle{
		{Font: driver.Font{Size: 13, Weight: 400}},
		{Font: driver.Font{Family: "Times New Roman", Size: 19, Weight: 400}},
		{Font: driver.Font{Size: 15, Weight: 400}, LineHeight: 20.3},
	} {
		st.Color = driver.RGB(0)
		for _, lines := range []int{1, 2, 3} {
			cut := measure(strings.Repeat("word ", 30), lines, st)
			uncut := measure(strings.TrimSuffix(strings.Repeat("word\n", lines), "\n"), 0, st)
			if cut.Height != uncut.Height {
				t.Errorf("%v: cut to %d line(s), %v tall; the lines uncut %v", st.Font, lines, cut.Height, uncut.Height)
			}
		}
		if st.LineHeight > 0 {
			if empty, line := measure("", 0, st), measure("word", 0, st); empty.Height != line.Height {
				t.Errorf("%v: an empty text %v tall, a line %v", st.Font, empty.Height, line.Height)
			}
		}
	}
}

// TestOneLineWithLeadingMatchesTheFramesetter checks texts in fonts with typographic leading, which a frame leaves out of its last line where CTLineGetTypographicBounds counts it: named fonts, so the test does not depend on the fallback a machine picks for Menlo. The ASCII texts are the ones that could take the one-line shortcut.
func TestOneLineWithLeadingMatchesTheFramesetter(t *testing.T) {
	defer func() { oneLineFast = true }()
	for _, family := range []string{"Hiragino Sans GB", "Heiti SC", "Thonburi", "Devanagari Sangam MN"} {
		for _, text := range []string{"abc", "Hello world ", "日本語のテキスト", "ภาษาไทย", "हिन्दी"} {
			for _, lines := range []int{0, 1} {
				at := driver.AttributedText{Text: text, MaxLines: lines,
					Style: driver.TextStyle{Font: driver.Font{Family: family, Size: 12, Weight: 400}, Color: driver.RGB(0)}}
				compareOneLine(t, fmt.Sprintf("%s/%q/%d", family, text, lines), &at)
			}
		}
	}
}

func compareOneLine(t *testing.T, name string, at *driver.AttributedText) {
	t.Helper()
	type result struct {
		auto, wide, narrow driver.Size
		pixels, cut        []byte
		lines              []driver.TextLine
		carets             []driver.Bounds
	}
	run := func(fast bool) result {
		oneLineFast = fast
		p := TextEngine{}.Prepare(at)
		defer p.Release()
		var r result
		r.auto = p.Measure(driver.Auto)
		r.wide = p.Measure(r.auto.Width + 40)
		// Too narrow: the framesetter wraps or cuts it either way.
		half := driver.Points(int(r.auto.Width/2) + 1)
		r.narrow = p.Measure(half)
		if r.narrow.Width > 0 && r.narrow.Height > 0 {
			img, _ := renderWith(t, p, driver.Size{Width: half, Height: r.narrow.Height + 2})
			r.cut = img.Pix
		}
		size := driver.Size{Width: r.auto.Width + 30, Height: r.auto.Height + 4}
		img, l := renderWith(t, p, size)
		r.pixels = img.Pix
		r.lines = l.TextLines()
		for i := 0; i <= len(at.Text); i++ {
			if rect, ok := l.CaretRect(driver.Caret{Offset: i}); ok {
				r.carets = append(r.carets, rect)
			}
		}
		return r
	}
	slow, fast := run(false), run(true)
	if slow.narrow != fast.narrow || !bytes.Equal(slow.cut, fast.cut) {
		t.Errorf("%s: a narrow box differs from the framesetter's", name)
	}
	if slow.auto != fast.auto || slow.wide != fast.wide {
		t.Errorf("%s: sizes %v %v, want the framesetter's %v %v", name, fast.auto, fast.wide, slow.auto, slow.wide)
	}
	if !bytes.Equal(slow.pixels, fast.pixels) {
		t.Errorf("%s: pixels differ from the framesetter's", name)
	}
	if !reflect.DeepEqual(slow.lines, fast.lines) {
		t.Errorf("%s: lines %+v, want %+v", name, fast.lines, slow.lines)
	}
	if !reflect.DeepEqual(slow.carets, fast.carets) {
		t.Errorf("%s: carets %v, want %v", name, fast.carets, slow.carets)
	}
}
