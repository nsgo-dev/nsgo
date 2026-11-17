package imagedecode

import (
	"bytes"
	"image"
	"image/png"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// FromPixels returns the pixels of an ImageSource, as the core copied them (driver.CopyPixels: at the origin, rows without padding), as a still image in layout: a copy, so the core's buffer stays as it was, and nothing decodes. Its size is a point a pixel.
func FromPixels(p *image.RGBA, layout Layout) *Image {
	w, h := p.Rect.Dx(), p.Rect.Dy()
	pix := make([]byte, w*h*4)
	if p.Stride == 4*w && p.Rect.Min == (image.Point{}) {
		copy(pix, p.Pix)
	} else {
		for y := range h {
			at := p.PixOffset(p.Rect.Min.X, p.Rect.Min.Y+y)
			copy(pix[y*w*4:(y+1)*w*4], p.Pix[at:at+w*4])
		}
	}
	if layout == BGRA {
		swapRB(pix)
	}
	return &Image{Width: w, Height: h, Size: driver.Size{Width: driver.Points(w), Height: driver.Points(h)},
		Frames: []Frame{{Pix: pix}}}
}

// AsPNG returns src with its Pixels, when it has them, as the Data of a PNG, for the backends that show images only from encoded bytes; it leaves any other source as it is. The PNG is written without compression, as it is decoded at once in the same process. Each call encodes anew: a backend keeps the PNG of the copy a view shows, which the core sends again when only the look changes.
func AsPNG(src driver.ImageSource) driver.ImageSource {
	if src.Pixels == nil {
		return src
	}
	p := src.Pixels
	src.Pixels, src.URI = nil, ""
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(&b, p); err != nil {
		return src
	}
	src.Data = b.Bytes()
	return src
}
