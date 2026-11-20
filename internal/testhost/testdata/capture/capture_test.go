package capture_test

import (
	"image"
	"testing"

	_ "github.com/nsgo-dev/nsgo"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

// TestCaptureShowsCompositedEffects checks that a macOS capture is what WindowServer composites: a filter and a box shadow are Core Animation's, and an AppKit redraw of the view tree leaves both out.
func TestCaptureShowsCompositedEffects(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("checks the macOS test host's capture")
	}
	const width, height = 300, 200
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Background: core.RGB(0xffffff), Padding: core.All(40), Gap: 40, Direction: core.Row}, Children: []core.Node{
			core.View{TestID: "inverted", Style: core.Style{Width: core.Pt(60), Height: core.Pt(60), Background: core.RGB(0xff0000), Filters: []core.Filter{core.Invert(1)}}},
			core.View{TestID: "shadowed", Style: core.Style{Width: core.Pt(40), Height: core.Pt(40), Background: core.RGB(0x0000ff),
				Shadows: []core.BoxShadow{{Offset: core.Point{Y: 30}, Color: core.RGB(0x000000)}}}},
		}}
	}, nsgotest.Size(width, height))
	img := app.Capture()
	at := func(x, y core.Points) (r, g, b uint32) {
		p := image.Pt(img.Bounds().Min.X+int(float64(x)*float64(img.Bounds().Dx())/width), img.Bounds().Min.Y+int(float64(y)*float64(img.Bounds().Dy())/height))
		r, g, b, _ = img.At(p.X, p.Y).RGBA()
		return r >> 8, g >> 8, b >> 8
	}
	// The colours are the display's, Display P3 on most Macs, so each check leaves room for the conversion.
	if r, g, b := at(70, 70); r > 160 || g < 200 || b < 200 {
		t.Errorf("the inverted red box shows %d,%d,%d, want cyan", r, g, b)
	}
	if r, g, b := at(160, 60); r > 40 || g > 40 || b < 200 {
		t.Errorf("the shadowed box shows %d,%d,%d, want blue", r, g, b)
	}
	if r, g, b := at(160, 95); r > 40 || g > 40 || b > 40 {
		t.Errorf("below the shadowed box shows %d,%d,%d, want its black shadow", r, g, b)
	}
}

// TestPartialBorderShowsOverABackdrop checks that a border on only some sides of a frosted view draws over its blur and tint, the right way up, as a uniform border does (#935).
func TestPartialBorderShowsOverABackdrop(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("checks the macOS test host's capture")
	}
	const width, height = 300, 300
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Background: core.RGB(0x2050c0), Padding: core.All(50)}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(200), Height: core.Pt(100), Background: core.RGBA(0xffffff, 0.3), BackdropFilters: []core.Filter{core.Blur(10)},
				Borders: core.Edges{Top: 6}, BorderColors: core.EdgeColors{Top: core.RGB(0xff0000)}}},
		}}
	}, nsgotest.Size(width, height))
	img := app.Capture()
	at := func(x, y core.Points) (r, g, b uint32) {
		p := image.Pt(img.Bounds().Min.X+int(float64(x)*float64(img.Bounds().Dx())/width), img.Bounds().Min.Y+int(float64(y)*float64(img.Bounds().Dy())/height))
		r, g, b, _ = img.At(p.X, p.Y).RGBA()
		return r >> 8, g >> 8, b >> 8
	}
	if r, g, b := at(150, 53); r < 180 || g > 110 || b > 110 {
		t.Errorf("the top border shows %d,%d,%d, want red over the backdrop", r, g, b)
	}
	if r, g, b := at(150, 147); r > 180 && g < 110 && b < 110 {
		t.Errorf("the bottom edge shows the top border's red %d,%d,%d: the ring is upside down", r, g, b)
	}
}
