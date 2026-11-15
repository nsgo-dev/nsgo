//go:build darwin && !ios

package macos

import (
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestInputLayoutAfterUnmount lays out an input's native view, single-line and multiline, after the backend has removed it, as AppKit can for a view an outgoing transition still holds.
func TestInputLayoutAfterUnmount(t *testing.T) {
	onMain(func() {
		loadAppKit()
		s := &surface{views: make([]*view, 2), animator: driver.NewAnimator(func(driver.Event) {})}
		for _, multiline := range []bool{false, true} {
			v := s.create(driver.KindInput, 1)
			s.views[v.id] = v
			v.configure(&driver.InputConfig{Multiline: multiline, Content: driver.Bounds{Size: driver.Size{Width: 200, Height: 60}}})
			native := v.native.Send(sel("retain"))
			s.delete(v, false)
			if views[native] != nil {
				t.Error("the removed input is still registered")
			}
			native.Send(sel("layout"))
			native.Send(selRelease)
		}
	})
}

// TestMultilineEditorScrollsUnderPadding checks that a multiline input's
// scroll view fills the padding box and its text keeps the padding, uneven
// sides included (#701).
func TestMultilineEditorScrollsUnderPadding(t *testing.T) {
	onMain(func() {
		loadAppKit()
		s := &surface{views: make([]*view, 2), animator: driver.NewAnimator(func(driver.Event) {})}
		v := s.create(driver.KindInput, 1)
		s.views[v.id] = v
		v.native.Send(selSetFrame, apple.Rect(0, 0, 300, 200))
		pad := driver.Edges{Top: 18, Right: 20, Bottom: 20, Left: 12}
		v.configure(&driver.InputConfig{Multiline: true, Padding: pad,
			Content: driver.Bounds{Origin: driver.Point{X: 13, Y: 19}, Size: driver.Size{Width: 266, Height: 160}}})
		v.native.Send(sel("layout"))
		in := v.input
		if got, want := objc.Send[apple.CGRect](in.host, sel("frame")), apple.Rect(1, 1, 298, 198); got != want {
			t.Errorf("scroll view at %v, want the padding box %v", got, want)
		}
		if got := objc.Send[apple.CGPoint](in.field, sel("textContainerOrigin")); got != (apple.CGPoint{X: 12, Y: 18}) {
			t.Errorf("text starts at %v, want after the top and left padding", got)
		}
		in.host.Send(sel("tile"))
		visible := objc.Send[apple.CGSize](in.host, sel("contentSize")).Width
		wraps := objc.Send[apple.CGSize](in.field.Send(sel("textContainer")), sel("size")).Width
		if wraps != visible-32 {
			t.Errorf("text wraps at %v pt in a %v pt viewport, want the left and right padding off", wraps, visible)
		}
		s.delete(v, false)
	})
}

// TestMultilineEditorWrapsInItsViewport checks a multiline input's text
// view, its container and the text it lays out against the width the
// scroll view shows, as the input narrows and widens, with overlay and
// legacy scrollers (#42).
func TestMultilineEditorWrapsInItsViewport(t *testing.T) {
	onMain(func() { multilineEditorWraps(t) })
}

func multilineEditorWraps(t *testing.T) {
	loadAppKit()
	text := strings.Repeat("多行输入在可见区域内换行。", 20) + " https://example.com/" + strings.Repeat("a", 300)
	for _, style := range []int{1, 0} { // overlay, legacy
		window := objc.ID(apple.Class("NSWindow")).Send(selAlloc).Send(sel("initWithContentRect:styleMask:backing:defer:"),
			apple.Rect(0, 0, 500, 200), uint(0), uint(2), false)
		window.Send(sel("setReleasedWhenClosed:"), false)
		scroll, tv := newMultilineEditor()
		window.Send(sel("contentView")).Send(selAddSubview, scroll)
		scroll.Send(sel("setScrollerStyle:"), style)
		tv.Send(sel("setString:"), apple.NSString(text))
		container := tv.Send(sel("textContainer"))
		layout := tv.Send(sel("layoutManager"))
		for _, width := range []float64{320, 185, 445, 60} {
			scroll.Send(sel("setFrame:"), apple.Rect(0, 0, width, 90))
			scroll.Send(sel("tile"))
			layout.Send(sel("ensureLayoutForTextContainer:"), container)
			visible := objc.Send[apple.CGSize](scroll, sel("contentSize")).Width
			editor := objc.Send[apple.CGRect](tv, sel("frame")).Size.Width
			wraps := objc.Send[apple.CGSize](container, sel("size")).Width
			used := objc.Send[apple.CGRect](layout, sel("usedRectForTextContainer:"), container)
			if editor != visible || wraps != visible || used.Origin.X+used.Size.Width > visible {
				t.Errorf("scroller style %d, %v pt wide: shows %v pt, editor %v pt, container %v pt, text to %v pt",
					style, width, visible, editor, wraps, used.Origin.X+used.Size.Width)
			}
		}
		scroll.Send(selRelease)
		window.Send(selRelease)
	}
}

// TestMultilineEditorLineHeightStylesTyping checks that an explicit line
// height reaches AppKit's typing attributes, which are what text inserted
// after the first layout inherits.
func TestMultilineEditorLineHeightStylesTyping(t *testing.T) {
	onMain(func() {
		loadAppKit()
		_, tv := newMultilineEditor()
		font := apple.Font(driver.Font{Size: 13})
		color := nsColor(driver.RGB(0x202020))
		setEditorParagraph(tv, objc.ID(font), color, 24, nsAlignCenter)
		attrs := tv.Send(sel("typingAttributes"))
		para := attrs.Send(sel("objectForKey:"), textAttrs().paragraph)
		if para == 0 {
			t.Fatal("typing attributes have no paragraph style")
		}
		if got := objc.Send[float64](para, sel("minimumLineHeight")); got != 24 {
			t.Errorf("minimum line height = %v, want 24", got)
		}
		if got := objc.Send[float64](para, sel("maximumLineHeight")); got != 24 {
			t.Errorf("maximum line height = %v, want 24", got)
		}
		if got := objc.Send[int](para, sel("alignment")); got != nsAlignCenter {
			t.Errorf("alignment = %d, want %d", got, nsAlignCenter)
		}
		apple.Release(font)
		tv.Send(selRelease)
	})
}
