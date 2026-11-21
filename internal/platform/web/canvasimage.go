//go:build js && wasm

package web

import (
	"runtime"
	"syscall/js"
	"weak"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvas2d"
)

func (s *surface) recordedCanvasImage(src driver.ImageSource, v *view) *canvasImage {
	key := src.Key()
	if im := s.canvasRecordings[key]; im != nil {
		return im
	}
	r := src.Recording
	canvas := js.Global().Get("document").Call("createElement", "canvas")
	pending := false
	canvas2d.PaintRecording(canvas, r, fontCSS, func(src driver.ImageSource) (js.Value, float64, bool) {
		im := s.loadImage(src, v)
		pending = pending || (!im.loaded && !im.failed)
		return im.img, im.density, im.loaded
	})
	if pending {
		return &canvasImage{}
	}
	im := &canvasImage{img: canvas, density: float64(r.PixelWidth) / float64(r.Size.Width), loaded: true}
	if s.canvasRecordings == nil {
		s.canvasRecordings = map[string]*canvasImage{}
	}
	s.canvasRecordings[key] = im
	owner := weak.Make(s)
	runtime.AddCleanup(r, func(key string) {
		s := owner.Value()
		if s == nil {
			return
		}
		// Go may run cleanups between callbacks. Dispatch through the page.
		var drop js.Func
		drop = js.FuncOf(func(js.Value, []js.Value) any {
			delete(s.canvasRecordings, key)
			drop.Release()
			return nil
		})
		js.Global().Call("queueMicrotask", drop)
	}, key)
	return im
}
