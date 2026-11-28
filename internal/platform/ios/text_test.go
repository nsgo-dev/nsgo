//go:build ios && nsgotest

package ios_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/ios"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestTextIsAsTallAsTheLinesItDraws checks that a Text in a box its first line fits only within the half point CoreText frames wrap past is as many lines tall as it draws: the Text of #2733, with a heavy, backgrounded span, and the same characters with no spans.
func TestTextIsAsTallAsTheLinesItDraws(t *testing.T) {
	const wait = 5 * time.Second
	const lineHeight = 23
	style := core.Style{FontSize: 16, LineHeight: lineHeight, Color: core.RGB(0x000000)}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, Background: core.RGB(0xffffff)}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(354)}, Children: []core.Node{
				core.Text{TestID: "spans", Style: style, Spans: []core.Span{
					{Content: "この俺を"},
					{Content: "差し置いた", Style: core.Style{FontWeight: core.FontWeight(900), Background: core.RGBA(0xffd60a, 0.45), CornerRadius: 3}},
					{Content: "剣聖の名を目指す者は 全て弓矢に遅れを取ることなしと そう断言するのだな"},
				}},
			}},
			core.View{Style: core.Style{Width: core.Pt(354)}, Children: []core.Node{
				core.Text{TestID: "plain", Style: style, Content: "この俺を差し置いた剣聖の名を目指す者は 全て弓矢に遅れを取ることなしと そう断言するのだな"},
			}},
		}}
	})
	for _, id := range []string{"spans", "plain"} {
		app.WaitFor("id:"+id, wait)
		var ink []float64
		var height float64
		var lines int
		app.WaitUntil(wait, func() bool {
			var ok bool
			ink, height, lines, ok = ios.TextInkForTest(id)
			return ok && len(ink) > 0
		})
		if want := float64(len(ink) * lineHeight); height != want || lines != len(ink) {
			t.Errorf("%s: %v pt tall with %d lines laid out, draws %d lines of ink at %v, want %v pt", id, height, lines, len(ink), ink, want)
		}
	}
}
