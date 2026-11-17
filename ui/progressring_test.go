package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestProgressRing reads its value as Progress does, and the spinners
// turn while work goes on.
func TestProgressRing(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
				ProgressRing{Label: "Upload", TestID: "upload", Value: func() float32 { return 40 }},
				Spinner{Label: "Sync", TestID: "ring"},
				Spinner{Label: "Load", TestID: "spokes", Variant: SpinnerSpokes, Size: 20},
			}}
		}, nsgotest.ManualFrames())
		if v := app.Find("upload").Props.Value; v.Now != 40 || v.Max != 100 {
			t.Fatalf("value = %v of %v, want 40 of 100", v.Now, v.Max)
		}
		if f := app.Find("spokes").Frame.Size; f.Width != 20 || f.Height != 20 {
			t.Fatalf("spokes are %v, want 20 pt across", f)
		}
		turn := func(id string) float32 { return app.Find(id).Animated.Rotate }
		before := []float32{turn("ring"), turn("spokes")}
		app.Advance(350 * time.Millisecond)
		if turn("ring") == before[0] || turn("spokes") == before[1] {
			t.Fatal("the spinners stand still")
		}
	})
}

func TestProgressRingTint(t *testing.T) {
	want := core.RGB(0x123456)
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return ProgressRing{TestID: "ring", Value: func() float32 { return 50 }, Tint: func() core.Color { return want }}
	})
	if got := canvasStrokeColor(app.Find("id:ring")); got != want {
		t.Fatalf("progress ring tint = %x, want %x", got, want)
	}
}

func TestButtonBusyUsesForegroundTint(t *testing.T) {
	app, colors := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return Button{Text: "Save", TestID: "save", Busy: func() bool { return true }}
	})
	button := app.Find("id:save")
	if !button.Props.Busy {
		t.Fatal("busy button is not reported busy")
	}
	if got := canvasStrokeColor(button); got != colors.OnAccent {
		t.Fatalf("busy button spinner tint = %x, want %x", got, colors.OnAccent)
	}
}

func canvasStrokeColor(v *nsgotest.View) core.Color {
	for _, child := range views(v) {
		if child.Kind != driver.KindCanvas {
			continue
		}
		var got core.Color
		for _, op := range child.Ops {
			if op.Kind == driver.DrawStrokeColor {
				got = op.Color
			}
		}
		if got != 0 {
			return got
		}
	}
	return 0
}
