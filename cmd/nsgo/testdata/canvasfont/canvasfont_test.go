package canvasfont

import (
	"image"
	"testing"
	"time"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"golang.org/x/image/font/gofont/gomono"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestCanvasAfterRegisterFont(t *testing.T) {
	if nsgotest.Target() != "web" {
		t.Skip("exercises asynchronous browser font loading")
	}
	var scope *core.Scope
	var again *core.Signal[int]
	var width float32
	draws := 0
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope, again = cx, core.NewSignal(cx, 0)
		return core.Canvas{Style: core.Style{Width: core.Pt(300), Height: core.Pt(100)}, Draw: func(d *core.Drawing, _ core.Size) {
			again.Get()
			draws++
			font := core.Font{Family: "Go Mono", Size: 40}
			width = d.TextMetrics("WWiiii", font).Width
			d.SetFillColor(core.RGB(0xffffff)).FillRect(0, 0, 300, 100)
			d.SetFillColor(core.RGB(0x000000)).FillText("WWiiii", 5, 60, font)
			// The measured geometry must change along with the glyphs.
			d.FillRect(5, 80, width, 4)
		}}
	}, nsgotest.Size(300, 100))
	before, initial := width, darkPixels(a.Capture())
	a.Post(func() {
		if err := scope.RegisterFont(gomono.TTF); err != nil {
			t.Fatal(err)
		}
	})
	a.WaitUntil(5*time.Second, func() bool { return width != before })
	loaded, measured, count := a.Capture(), width, draws
	a.Post(func() { again.Set(1) })
	a.Settle()
	forced := a.Capture()
	if darkPixels(loaded) == initial {
		t.Fatal("the loaded font did not change the canvas pixels")
	}
	if measured != width || !samePixels(loaded, forced) {
		t.Fatalf("only an unrelated redraw updated the canvas: width %v -> %v, dark pixels %d -> %d", measured, width, darkPixels(loaded), darkPixels(forced))
	}
	t.Logf("font arrival: draws=%d, width=%v -> %v, dark pixels=%d -> %d", count, before, measured, initial, darkPixels(loaded))
}

func samePixels(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}

func darkPixels(img image.Image) (n int) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r+g+b < 3*0x8000 {
				n++
			}
		}
	}
	return n
}
