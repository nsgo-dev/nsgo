//go:build darwin || ios

package apple

import (
	"runtime"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	cgBitmapContextCreateImage func(uintptr) uintptr
	canvasRecordings           = map[string]*loadedImage{}
)

func recordedCanvasImage(src driver.ImageSource, host CanvasHost) *loadedImage {
	key := src.Key()
	if img := canvasRecordings[key]; img != nil {
		return img
	}
	r := src.Recording
	bitmapsLoaded.Do(loadBitmaps)
	const premultipliedFirst, byteOrder32Little = 2, 2 << 12
	w, h := r.PixelWidth, r.PixelHeight
	ctx := cgBitmapContextCreate(0, uintptr(w), uintptr(h), 8, uintptr(w*4), srgb, premultipliedFirst|byteOrder32Little)
	img := &loadedImage{width: float64(w), height: float64(h), points: r.Size}
	if ctx != 0 {
		defer Release(ctx)
		scaleX := float64(w) / float64(r.Size.Width)
		scaleY := float64(h) / float64(r.Size.Height)
		host.Ratio = max(scaleX, scaleY)
		pending := false
		DrawTopDown(ctx, float64(h), true, func(baseUp bool) {
			cgContextScaleCTM(ctx, scaleX, scaleY)
			host.BaseUp = baseUp
			p := newPlayer(ctx, r.Ops, driver.CanvasSplit{}, host)
			p.run()
			pending = p.waited
		})
		if pending {
			return nil
		}
		img.image = cgBitmapContextCreateImage(ctx)
	}
	img.failed = img.image == 0
	canvasRecordings[key] = img
	runtime.AddCleanup(r, releaseCanvasRecording, key)
	return img
}

func releaseCanvasRecording(key string) {
	OnMain(func() {
		if img := canvasRecordings[key]; img != nil {
			Release(img.image)
			delete(canvasRecordings, key)
		}
	})
}
