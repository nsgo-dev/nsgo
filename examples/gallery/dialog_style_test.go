package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

// TestDialogStyleUpdatesOpenSurface checks the displayed pixels as well as the panel's batch when its theme changes while it remains open.
func TestDialogStyleUpdatesOpenSurface(t *testing.T) {
	for _, form := range []struct {
		name  string
		idiom core.Idiom
	}{{"centered", core.IdiomDesktop}, {"sheet", core.IdiomPhone}} {
		t.Run(form.name, func(t *testing.T) {
			e := core.Environment{OS: core.OSMacOS, Idiom: form.idiom, Locale: "en-US", ReduceMotion: true}
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				colors := ui.ColorsOf(cx)
				return ui.Dialog{Open: func() bool { return true }, TestID: "panel",
					Style: core.Style{Width: core.Pt(240)},
					StyleFunc: func() core.Style {
						return core.Style{Background: colors.Surface()}
					},
					Children: []core.Node{core.View{TestID: "probe", Style: core.Style{Width: core.Pt(120), Height: core.Pt(60)}}}}
			}, nsgotest.Environment(e), nsgotest.Size(400, 400))
			id := app.Find("panel").ID
			for _, dark := range []bool{false, true, false} {
				e.Dark = dark
				app.SetEnvironment(e)
				palette := core.DefaultTheme().Light
				if dark {
					palette = core.DefaultTheme().Dark
				}
				panel := app.Find("panel")
				if panel.ID != id || panel.Style.Background != palette.Surface {
					t.Fatalf("open panel changed identity or kept its old background: %v", panel)
				}
				if nsgotest.Target() == "" {
					continue
				}
				probe := app.Find("probe").Bounds()
				img := app.Capture()
				x := int(float32(img.Bounds().Dx()) * float32(probe.Origin.X+probe.Size.Width/2) / 400)
				y := int(float32(img.Bounds().Dy()) * float32(probe.Origin.Y+probe.Size.Height/2) / 400)
				r, g, b, _ := img.At(x, y).RGBA()
				wr, wg, wb, _ := palette.Surface.Components()
				for i, got := range []uint32{r, g, b} {
					want := []float64{wr, wg, wb}[i] * 255
					if delta := float64(got>>8) - want; delta < -3 || delta > 3 {
						t.Fatalf("dark=%v: displayed pixel at %d,%d = %d,%d,%d, want %v", dark, x, y, r>>8, g>>8, b>>8, palette.Surface)
					}
				}
			}
		})
	}
}
