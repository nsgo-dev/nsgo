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
					Children: []core.Node{core.View{TestID: "probe", Style: core.Style{Width: core.Pt(120), Height: core.Pt(60)}, Children: []core.Node{
						// Recreate a reference swatch in each appearance. Comparing pixels in one capture also works in a display's ICC space, which image/png does not convert to sRGB.
						core.Dynamic(func(*core.Scope) core.Node {
							return core.View{TestID: "reference", Style: core.Style{Width: core.Pt(20), Height: core.Pt(20), Background: colors.Surface()}}
						}),
					}}}}
			}, nsgotest.Environment(e), nsgotest.Size(400, 400))
			id := app.Find("panel").ID
			// The swatch must change with the appearance, or a capture that kept the old frame would match it.
			var last uint32
			for step, dark := range []bool{false, true, false} {
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
				pixel := func(bounds core.Bounds) (int, int) {
					return int(float32(img.Bounds().Dx()) * float32(bounds.Origin.X+bounds.Size.Width/2) / 400),
						int(float32(img.Bounds().Dy()) * float32(bounds.Origin.Y+bounds.Size.Height/2) / 400)
				}
				x, y := pixel(probe)
				r, g, b, _ := img.At(x, y).RGBA()
				rx, ry := pixel(app.Find("reference").Bounds())
				wr, wg, wb, _ := img.At(rx, ry).RGBA()
				// Light and dark surfaces differ by far more than any display profile shifts a channel.
				luma := (wr + wg + wb) / 3 >> 8
				if step > 0 && max(luma, last)-min(luma, last) < 64 {
					t.Fatalf("dark=%v: the reference swatch stayed at %d,%d,%d after the appearance changed", dark, wr>>8, wg>>8, wb>>8)
				}
				last = luma
				for i, got := range []uint32{r, g, b} {
					want := []uint32{wr, wg, wb}[i]
					if delta := int(got>>8) - int(want>>8); delta < -3 || delta > 3 {
						t.Fatalf("dark=%v: displayed pixel at %d,%d = %d,%d,%d, reference = %d,%d,%d", dark, x, y, r>>8, g>>8, b>>8, wr>>8, wg>>8, wb>>8)
					}
				}
			}
		})
	}
}
