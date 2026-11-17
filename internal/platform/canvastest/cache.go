package canvastest

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// CacheSources returns distinct, tiny red images for cache-pressure tests.
// Their data stays owned by the caller, so address-based source keys stay unique.
func CacheSources(n int) []driver.ImageSource {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	sources := make([]driver.ImageSource, n)
	for i := range sources {
		sources[i].Data = bytes.Clone(b.Bytes())
	}
	return sources
}
