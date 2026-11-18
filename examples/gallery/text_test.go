package main

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

//go:embed resources/fonts/Pacifico-Regular.ttf
var pacifico []byte

// TestARegisteredFontMeasures registers a font as the app runs, under nsgo test -target, and lays out a text in it with its own glyphs, not the system font's that a family it cannot find falls back to (#313).
func TestARegisteredFontMeasures(t *testing.T) {
	if nsgotest.Target() == "" {
		t.Skip("a headless test's platform registers no fonts")
	}
	const words = "Registered fonts"
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		if err := cx.RegisterFont(pacifico); err != nil {
			t.Errorf("RegisterFont: %v", err)
		}
		return core.View{Style: core.Style{AlignItems: core.AlignStart}, Children: []core.Node{
			core.Text{TestID: "registered", Content: words, Style: core.Style{FontFamily: "Pacifico", FontSize: 20}},
			core.Text{TestID: "missing", Content: words, Style: core.Style{FontFamily: "NSGo Missing Family", FontSize: 20}},
		}}
	})
	size := func(id string) driver.Size { return app.Find("id:" + id).Frame.Size }
	// The web loads a face after RegisterFont returns.
	deadline := time.Now().Add(5 * time.Second)
	for app.Settle(); size("registered") == size("missing"); app.Settle() {
		if time.Now().After(deadline) {
			t.Fatalf("the registered font's text measures %v, as the fallback's does", size("registered"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSelectingInAScrollViewKeepsItsPlace selects inside a scroll view under nsgo test -target and leaves it where it was: only a drag past its edge scrolls it, never where the last drag ended before this one's first move.
func TestSelectingInAScrollViewKeepsItsPlace(t *testing.T) {
	if nsgotest.Target() == "" {
		t.Skip("a headless drag is a finger's, which scrolls")
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var texts []core.Node
		for i := range 40 {
			texts = append(texts, core.Text{TestID: fmt.Sprint("line-", i), Content: fmt.Sprint(i, " ", strings.Repeat("Selectable words ", 8))})
		}
		// A bar under the scroll view, as a chat's composer, puts the window's corner past its edge, and its spinner keeps the frame clock running.
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			core.Scroll{TestID: "scroll", Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0)}, Children: []core.Node{
				core.View{Selectable: true, Style: core.Style{Padding: core.All(16), Gap: 12}, Children: texts},
			}},
			core.View{Style: core.Style{Height: core.Pt(40), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{ui.Spinner{Size: 16}}},
		}}
	}, nsgotest.Size(400, 500))
	app.Scroll("id:scroll", 600)
	offset := func() core.Points { return app.Find("id:scroll").Offset.Y }
	at := offset()
	line := ""
	for i := range 40 {
		if y := app.Find(fmt.Sprint("id:line-", i)).Bounds().Origin.Y; y > 150 && y < 250 {
			line = fmt.Sprint("id:line-", i)
			break
		}
	}
	for range 2 {
		app.Drag(line, core.Point{X: 10, Y: 10}, core.Point{X: 20, Y: -20}, core.Point{X: 30, Y: -40})
		if got := offset(); got != at {
			t.Fatalf("selecting scrolled the view from %v to %v", at, got)
		}
	}
}

// TestSelectingReportsTheText drags across a text under nsgo test -target and has the area hear the text selected as it grows within the text, with the box of the text, which stays the same (the range's box on the web), and nothing once Escape clears it.
func TestSelectingReportsTheText(t *testing.T) {
	if nsgotest.Target() == "" {
		t.Skip("a headless drag is a finger's, which scrolls")
	}
	var mu sync.Mutex
	var heard []core.TextSelection
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16)}, Children: []core.Node{
			core.View{TestID: "area", Selectable: true, OnTextSelection: func(s core.TextSelection) {
				mu.Lock()
				heard = append(heard, s)
				mu.Unlock()
			}, Children: []core.Node{
				core.Text{TestID: "line", Content: "serendipity happens every day", Style: core.Style{FontSize: 20}},
			}},
		}}
	}, nsgotest.Size(400, 200))
	box := app.Find("id:line").Bounds().Size
	app.DragAt("id:line", core.Point{X: 2, Y: box.Height / 2}, core.Point{X: 60}, core.Point{X: 120}, core.Point{X: box.Width - 4})
	app.Settle()
	mu.Lock()
	got := slices.Clone(heard)
	mu.Unlock()
	var texts []string
	for _, s := range got {
		texts = append(texts, s.Text)
		if web := nsgotest.Target() == "web"; !web && s.Bounds.Size != box || web && (s.Bounds.Size.Width > box.Width || s.Bounds.Size.Height > box.Height) {
			t.Errorf("heard %+v, want the text's box %v", s, box)
		}
	}
	if len(texts) < 2 || texts[len(texts)-1] != "serendipity happens every day" {
		t.Fatalf("heard %q as the drag grew within one text, want it to end with the whole text", texts)
	}
	app.Key("Escape")
	app.Settle()
	mu.Lock()
	last := heard[len(heard)-1]
	mu.Unlock()
	if last.Text != "" || last.Bounds.Size != (core.Size{}) {
		t.Errorf("after Escape heard %+v, want no selection", last)
	}
}
