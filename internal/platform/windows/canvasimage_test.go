//go:build windows

package windows

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

func TestCanvasRecordingRendersIntoDirect2DBitmap(t *testing.T) {
	g, err := devices()
	if err != nil {
		t.Skip(err)
	}
	w := &window{scale: canvastest.Scale, animator: driver.NewAnimator(nil)}
	defer w.closeCanvasImages()
	d := driver.NewDrawing(nil).SetFillColor(driver.RGB(0x1133ff)).FillRect(0, 0, 40, 20).
		SetFillColor(driver.RGB(0xee7700)).FillRect(0, 0, 20, 10)
	src := driver.NewCanvasRecording(d.Ops, driver.Size{Width: 40, Height: 20}, 2)
	frame := driver.NewDrawing(nil).DrawImage(src, 2, 2).
		DrawImageFrom(src, 0, 0, 20, 10, 48, 4, 20, 10)
	draw := func(p *painter) {
		p.canvas(&view{drawing: frame}, 72, 26)
	}
	img := drawImage(t, g, 72, 26, draw, w)
	for _, point := range []struct {
		x, y int
		rgb  uint32
	}{
		{4, 4, 0xee7700},
		{30, 18, 0x1133ff},
		{51, 8, 0xee7700},
	} {
		p := img.RGBAAt(point.x*canvastest.Scale, point.y*canvastest.Scale)
		got := uint32(p.R)<<16 | uint32(p.G)<<8 | uint32(p.B)
		if got != point.rgb {
			t.Fatalf("pixel %d,%d: %#06x, want %#06x", point.x, point.y, got, point.rgb)
		}
	}
	previous := w.canvasRecordings[src.Key()].bitmap
	drawImage(t, g, 72, 26, draw, w)
	if w.canvasRecordings[src.Key()].bitmap != previous {
		t.Fatal("the next frame replaced the retained Direct2D bitmap")
	}
}
