//go:build linux && !android

package linux

import (
	"runtime"
	"weak"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func (w *window) recordedCanvasImage(src driver.ImageSource, v *view) *imageSlot {
	key := src.Key()
	if s := w.canvasRecordings[key]; s != nil {
		return s
	}
	r := src.Recording
	slot := &imageSlot{src: driver.ImageSource{Size: r.Size}}
	surf := cairoImageSurfaceCreate(0, int32(r.PixelWidth), int32(r.PixelHeight))
	if surf != 0 {
		cr := cairoCreate(surf)
		width, height := float64(r.Size.Width), float64(r.Size.Height)
		cairoScale(cr, float64(r.PixelWidth)/width, float64(r.PixelHeight)/height)
		waited := w.playCanvasFrom(v, cr, rect{w: width, h: height}, r.Ops, driver.CanvasSplit{}, nil)
		cairoDestroy(cr)
		if waited {
			cairoSurfaceDestroy(surf)
			return nil
		}
		cairoSurfaceFlush(surf)
		slot.pic = &picture{width: r.PixelWidth, height: r.PixelHeight}
		slot.surface = surf
	}
	if w.canvasRecordings == nil {
		w.canvasRecordings = map[string]*imageSlot{}
	}
	w.canvasRecordings[key] = slot
	owner := weak.Make(w)
	runtime.AddCleanup(r, func(key string) {
		if w := owner.Value(); w != nil {
			w.post(func() {
				if slot := w.canvasRecordings[key]; slot != nil {
					slot.reset(driver.ImageSource{})
					delete(w.canvasRecordings, key)
				}
			})
		}
	}, key)
	return slot
}
