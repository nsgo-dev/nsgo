package android

import (
	"image/color"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestTabBarSVGNativeTint exercises the real CanvasNode replay and ImageView tint, which headless props alone cannot verify: the SVG stands in for an icon name Android has no drawable for, as an SF Symbol's shared with iOS, and a name that does name one keeps its drawable.
func TestTabBarSVGNativeTint(t *testing.T) {
	if nsgotest.Target() != "android" {
		t.Skip("requires Android pixels")
	}
	svg, err := core.ParseSVG([]byte(`<svg viewBox="0 0 24 24"><rect width="24" height="24" fill="currentColor"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	var choose func(int)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		theme := core.DefaultTheme()
		theme.Light.Accent, theme.Dark.Accent = core.RGB(0xd02030), core.RGB(0xd02030)
		cx.SetTheme(theme)
		selected := core.NewSignal(cx, 0)
		choose = selected.Set
		return core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}, TestID: "root", Children: []core.Node{
			core.TabBar{Items: []core.TabBarItem{{Title: "Home", SVG: svg}, {Title: "Search", Icon: "magnifyingglass", SVG: svg}, {Title: "Menu", Icon: "android:ic_menu_search", SVG: svg}},
				Selected: selected.Get, OnChange: selected.Set, TestID: "bar",
				Style: core.Style{Width: core.Pt(360), Height: core.Pt(80), Margin: core.All(60)}},
		}}
	})
	isIdle := func(c color.NRGBA) bool {
		return c == (color.NRGBA{R: 0x49, G: 0x45, B: 0x4f, A: 0xff}) || c == (color.NRGBA{R: 0xca, G: 0xc4, B: 0xd0, A: 0xff})
	}
	check := func(selected int) (idle [3]int) {
		t.Helper()
		app.Settle()
		img := app.Capture()
		scale := float64(img.Bounds().Dx()) / float64(app.Find("id:root").Frame.Size.Width)
		bar := app.Find("id:bar").Bounds()
		third := float64(bar.Size.Width) / 3
		for i := range 2 {
			x := int((float64(bar.Origin.X) + (float64(i)+0.5)*third) * scale)
			y := int((float64(bar.Origin.Y) + 24) * scale)
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			accent := got.R == 0xd0 && got.G == 0x20 && got.B == 0x30 && got.A == 0xff
			if i == selected && !accent || i != selected && !isIdle(got) {
				t.Fatalf("tab %d with selection %d: pixel %v at %d,%d", i, selected, got, x, y)
			}
		}
		// The idle-tinted pixels of each tab's column, its icon's and its title's.
		for i := range 3 {
			x0, x1 := int((float64(bar.Origin.X)+float64(i)*third)*scale), int((float64(bar.Origin.X)+float64(i+1)*third)*scale)
			y0, y1 := int(float64(bar.Origin.Y)*scale), int(float64(bar.Origin.Y+bar.Size.Height)*scale)
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					if isIdle(color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)) {
						idle[i]++
					}
				}
			}
		}
		return idle
	}
	idle := check(0)
	// Both idle tabs carry the same filled square SVG; the system's search drawable, which the third names, covers far less of its square.
	if idle[2]*2 > idle[1] {
		t.Fatalf("idle pixels: %d under the SVG, %d where Icon names a drawable; the drawable did not take precedence", idle[1], idle[2])
	}
	app.Post(func() { choose(1) })
	check(1)
}
