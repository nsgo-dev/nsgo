//go:build js && wasm

package canvas2d

import (
	"math"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// PaintRecording renders a standalone canvas image in a fresh 2D context.
func PaintRecording(canvas js.Value, r *driver.CanvasRecording, font func(driver.Font) string,
	image func(driver.ImageSource) (js.Value, float64, bool)) {
	canvas.Set("width", r.PixelWidth)
	canvas.Set("height", r.PixelHeight)
	w, h := float64(r.Size.Width), float64(r.Size.Height)
	sx, sy := float64(r.PixelWidth)/w, float64(r.PixelHeight)/h
	ctx := canvas.Call("getContext", "2d")
	ctx.Call("scale", sx, sy)
	p := NewPlayer(ctx, max(sx, sy), math.Hypot(w, h))
	p.baseX, p.baseY = sx, sy
	p.Font, p.Image = font, image
	for i := range r.Ops {
		p.Play(&r.Ops[i])
	}
}
