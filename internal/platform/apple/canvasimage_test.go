//go:build darwin && !ios

package apple

import (
	"image"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

func TestCanvasRecordingReusesTheRasterAndKeepsItsSize(t *testing.T) {
	size := driver.Size{Width: 40, Height: 20}
	first := driver.NewDrawing(nil).SetFillColor(driver.RGB(0x1133ff)).FillRect(0, 0, 40, 20).
		SetFillColor(driver.RGB(0xee7700)).FillRect(0, 0, 20, 10)
	source := driver.NewCanvasRecording(first.Ops, size, 2)
	draw := driver.NewDrawing(nil).DrawImage(source, 2, 2).DrawImageFrom(source, 0, 0, 20, 10, 48, 4, 20, 10)
	pixels := play(t, "canvas-recording", draw, 72, 26, false)
	check := func(img *image.RGBA, x, y int, rgb uint32) {
		t.Helper()
		p := img.RGBAAt(x*canvastest.Scale, y*canvastest.Scale)
		got := uint32(p.R)<<16 | uint32(p.G)<<8 | uint32(p.B)
		if got != rgb {
			t.Fatalf("pixel at %d,%d: %#06x, want %#06x", x, y, got, rgb)
		}
	}
	check(pixels, 4, 4, 0xee7700)
	check(pixels, 30, 18, 0x1133ff)
	check(pixels, 51, 8, 0xee7700)
	im := canvasImage(source, CanvasHost{Ratio: 1})
	play(t, "canvas-recording-redraw", draw, 72, 26, false)
	if got := canvasImage(source, CanvasHost{Ratio: 3}); got != im {
		t.Fatal("a new frame or screen scale replaced the kept raster")
	}

	second := driver.NewDrawing(nil).SetFillColor(driver.RGB(0x228833)).FillRect(0, 0, 40, 20)
	replacement := driver.NewCanvasRecording(second.Ops, size, 2)
	next := driver.NewDrawing(nil).DrawImage(replacement, 2, 2)
	check(play(t, "canvas-recording-replaced", next, 44, 24, false), 4, 4, 0x228833)
	if canvasImage(replacement, CanvasHost{Ratio: 1}) == im {
		t.Fatal("the new recording reused the previous image")
	}
}

func BenchmarkCanvasRecordedTileLayer(b *testing.B) {
	const side = 256
	tiles := driver.NewDrawing(nil)
	for y := range side / 8 {
		for x := range side / 8 {
			c := driver.RGB(0x127031)
			if (x+y)%2 == 0 {
				c = driver.RGB(0x3a7040)
			}
			tiles.SetFillColor(c).FillRect(float32(x*8), float32(y*8), 8, 8)
		}
	}
	src := driver.NewCanvasRecording(tiles.Ops, driver.Size{Width: side, Height: side}, 1)
	kept := driver.NewDrawing(nil).DrawImage(src, 0, 0)
	for _, render := range []struct {
		name string
		d    *driver.Drawing
	}{
		{"replay", tiles},
		{"recorded", kept},
	} {
		b.Run(render.name, func(b *testing.B) {
			ctx, img := bitmap(side, side)
			defer Release(ctx)
			draw := func() {
				DrawTopDown(ctx, side, true, func(up bool) {
					Play(ctx, render.d.Ops, CanvasHost{Ratio: canvastest.Scale, BaseUp: up})
				})
			}
			draw()
			for b.Loop() {
				draw()
			}
			b.ReportMetric(float64(len(render.d.Ops)), "ops/frame")
			if img.Pix[3] == 0 {
				b.Fatal("bitmap was not painted")
			}
		})
	}
}
