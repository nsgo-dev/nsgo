package android

import (
	"image/color"
	"math"
	"sync/atomic"
	"testing"
	"time"

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
		choose = func(i int) { cx.Post(func() { selected.Set(i) }) }
		return core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), Padding: core.All(30)}, TestID: "root", Children: []core.Node{
			core.TabBar{Items: []core.TabBarItem{{Title: "Home", SVG: svg}, {Title: "Search", Icon: "magnifyingglass", SVG: svg}, {Title: "Menu", Icon: "android:ic_menu_search", SVG: svg}},
				Selected: selected.Get, OnChange: selected.Set, TestID: "bar",
				Style: core.Style{Width: core.Pct(100), Height: core.Pt(80)}},
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

// A bar only reserves the part of its own frame under the navigation bar.
// Moving an ancestor must update that overlap even if the bar's local frame
// stays unchanged, and moving back must restore its original natural height.
func TestTabBarNativeInsetsFollowPlacement(t *testing.T) {
	if nsgotest.Target() != "android" {
		t.Skip("requires Android pixels")
	}
	svg, err := core.ParseSVG([]byte(`<svg viewBox="0 0 24 24"><rect width="24" height="24" fill="currentColor"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	var move func(int)
	var safe atomic.Uint32
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		theme := core.DefaultTheme()
		theme.Light.Accent, theme.Dark.Accent = core.RGB(0xd02030), core.RGB(0xd02030)
		cx.SetTheme(theme)
		placement := core.NewSignal(cx, 0)
		move = func(at int) { cx.Post(func() { placement.Set(at) }) }
		env := cx.Env()
		return core.View{TestID: "root", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				bottom := env.SafeArea().Bottom
				safe.Store(math.Float32bits(float32(bottom)))
				st := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0)}
				switch placement.Get() {
				case 1:
					st.Bottom = core.Pt(0)
				case 2:
					st.Bottom = core.Pt(bottom)
				case 3:
					st.Bottom = core.Pt(bottom / 2)
				default:
					st.Top = core.Pt(60)
				}
				return st
			}, Children: []core.Node{core.TabBar{TestID: "bar", Items: []core.TabBarItem{{Title: "Home", SVG: svg}, {Title: "Search", SVG: svg}}}}},
		}}
	})
	for _, at := range []int{0, 1, 2, 3, 0} {
		move(at)
		app.Settle()
		inset := core.Points(math.Float32frombits(safe.Load()))
		if inset <= 0 {
			t.Skip("requires a bottom navigation inset")
		}
		want := core.Points(80)
		if at == 1 {
			want += inset
		}
		if at == 3 {
			want += inset / 2
		}
		app.WaitUntil(5*time.Second, func() bool { return math.Abs(float64(app.Find("id:bar").Bounds().Size.Height-want)) < .51 })
		img := app.Capture()
		scale := float64(img.Bounds().Dx()) / float64(app.Find("id:root").Frame.Size.Width)
		bar := app.Find("id:bar").Bounds()
		for tab := range 2 {
			ink := func(c color.NRGBA) bool {
				if tab == 0 {
					return c == (color.NRGBA{R: 0xd0, G: 0x20, B: 0x30, A: 255})
				}
				return c == (color.NRGBA{R: 0x49, G: 0x45, B: 0x4f, A: 255}) || c == (color.NRGBA{R: 0xca, G: 0xc4, B: 0xd0, A: 255})
			}
			x := int((float64(bar.Origin.X) + (float64(tab)+.5)*float64(bar.Size.Width)/2) * scale)
			y := int((float64(bar.Origin.Y) + 24) * scale)
			if c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA); !ink(c) {
				t.Fatalf("placement %d tab %d icon pixel %v", at, tab, c)
			}
			labels := 0
			for y := int((float64(bar.Origin.Y) + 48) * scale); y < int((float64(bar.Origin.Y)+68)*scale); y++ {
				for x := int((float64(bar.Origin.X) + float64(tab)*float64(bar.Size.Width)/2) * scale); x < int((float64(bar.Origin.X)+float64(tab+1)*float64(bar.Size.Width)/2)*scale); x++ {
					if ink(color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)) {
						labels++
					}
				}
			}
			if labels < 5 {
				t.Fatalf("placement %d tab %d label clipped (%d pixels)", at, tab, labels)
			}
		}
	}
}
