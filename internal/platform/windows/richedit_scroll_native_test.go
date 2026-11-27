//go:build windows

package windows

import (
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A pasted reply and caret navigation must leave the last page filled.
func TestRichEditPasteKeepsLastPageVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	ready := make(chan struct{}, 1)
	hwnd := startApp(t, "nsgo-paste-last-page", func(cx *core.Scope) core.Node {
		text := core.NewSignal(cx, "")
		return core.Input{Value: text.Get, OnChange: text.Set, Multiline: true, AutoFocus: true,
			OnFocus: func() {
				select {
				case ready <- struct{}{}:
				default:
				}
			},
			Style: core.Style{Width: driver.Pt(480), Height: driver.Pt(240), FontSize: 16, LineHeight: 24}}
	}, 520, 420)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("field did not gain focus")
	}
	onMain(t, hwnd, func(w *window) {
		in := w.focusedField()
		if in == nil || in.ensureRichEdit() == nil {
			t.Error("focused field did not initialize RichEdit")
			return
		}
		check := func(stage string) {
			t.Helper()
			w.redrawAll()
			w.paint()
			pos, height, ok := in.re.vScrollMetrics()
			if !ok {
				t.Errorf("%s: no scroll metrics", stage)
				return
			}
			maximum := max(height-in.height, 0)
			// RichEdit reserves the caret's descent below the final text line.
			if pos > maximum+float32(in.config.Style.LineHeight) || pos < 0 {
				t.Errorf("%s: offset %.1f exceeds last page %.1f (content %.1f, viewport %.1f)", stage, pos, maximum, height, in.height)
			}
		}
		// insert is the native clipboard path after Unicode text is read.
		in.insert(strings.Repeat("## Reply\n\n多行内容应保留换行，并填满可视区域。\n\n", 30) + "Last line.")
		check("paste")
		w.editKey(vkHome, true, false, false)
		check("home")
		if pos, _, _ := in.re.vScrollMetrics(); pos != 0 {
			t.Errorf("home: offset %.1f, want zero", pos)
		}
		w.editKey(vkEnd, true, false, false)
		check("end")
		for i := 0; i < 4; i++ {
			before, _, _ := in.re.vScrollMetrics()
			if !in.wheelScroll(1) {
				t.Errorf("wheel %d: did not move from %.1f", i, before)
			}
			after, _, _ := in.re.vScrollMetrics()
			if after >= before {
				t.Errorf("wheel %d: scrolling upward moved from %.1f to %.1f", i, before, after)
			}
			for frame := 0; frame < 3; frame++ {
				w.redrawAll()
				w.paint()
				if pos, _, _ := in.re.vScrollMetrics(); pos != after {
					t.Errorf("wheel %d frame %d: offset rebounded from %.1f to %.1f", i, frame, after, pos)
				}
			}
		}
		in.config.Content.Size.Height = 360
		check("grow viewport")
		in.config.Content.Size.Height = 120
		in.re.scrollCaret()
		check("shrink viewport")
	})
}
