//go:build js && wasm

package web

import (
	goimage "image"
	"slices"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// pixelsCanvas returns a <canvas> holding p's pixels, as the core copied them: canvases draw from it, and an image view shows the PNG it encodes, so the page links no encoder of Go's.
func pixelsCanvas(p *goimage.RGBA) js.Value {
	w, h := p.Rect.Dx(), p.Rect.Dy()
	// ImageData's alpha is straight; image.RGBA's is premultiplied.
	straight := make([]byte, w*h*4)
	for i := 0; i+3 < len(straight) && i+3 < len(p.Pix); i += 4 {
		a := uint32(p.Pix[i+3])
		straight[i+3] = byte(a)
		if a == 0 {
			continue
		}
		for c := range 3 {
			straight[i+c] = byte(min(uint32(p.Pix[i+c])*255/a, 255))
		}
	}
	g := js.Global()
	data := g.Get("Uint8ClampedArray").New(len(straight))
	js.CopyBytesToJS(data, straight)
	canvas := g.Get("document").Call("createElement", "canvas")
	canvas.Set("width", w)
	canvas.Set("height", h)
	if w > 0 && h > 0 {
		canvas.Call("getContext", "2d").Call("putImageData", g.Get("ImageData").New(data, w, h), 0, 0)
	}
	return canvas
}

// pixelsURL returns a data URL of a PNG of p's pixels, which the browser encodes.
func pixelsURL(p *goimage.RGBA) string {
	return pixelsCanvas(p).Call("toDataURL", "image/png").String()
}

// maxCanvasFrames and maxCanvasFramePixels bound the canvases a surface keeps of the pixels its canvases draw, by count and by their pixels together (8 Mi, four 1920x1080 frames): a stream's older versions are never drawn again.
const (
	maxCanvasFrames      = 8
	maxCanvasFramePixels = 1 << 23
)

func pixelCount(p *goimage.RGBA) int { return p.Rect.Dx() * p.Rect.Dy() }

type canvasFrame struct {
	pixels *goimage.RGBA
	image  *canvasImage
}

// canvasFrame returns src's pixels in a canvas of their own, made at once. The surface keeps the latest few, by the core's copy, the most recently drawn last: a canvas that draws again at the same versions makes none anew, and the rest go, as each version is a copy of its own.
func (s *surface) canvasFrame(src driver.ImageSource) *canvasImage {
	for i, f := range s.canvasFrames {
		if f.pixels == src.Pixels {
			s.canvasFrames = append(append(s.canvasFrames[:i:i], s.canvasFrames[i+1:]...), f)
			return f.image
		}
	}
	im := &canvasImage{img: pixelsCanvas(src.Pixels), density: 1, loaded: true}
	for len(s.canvasFrames) > 0 && (len(s.canvasFrames) >= maxCanvasFrames || s.canvasFramePixels+pixelCount(src.Pixels) > maxCanvasFramePixels) {
		s.canvasFramePixels -= pixelCount(s.canvasFrames[0].pixels)
		s.canvasFrames = slices.Delete(s.canvasFrames, 0, 1)
	}
	s.canvasFramePixels += pixelCount(src.Pixels)
	s.canvasFrames = append(s.canvasFrames, canvasFrame{src.Pixels, im})
	return im
}
