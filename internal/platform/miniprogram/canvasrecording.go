//go:build js && wasm

package miniprogram

import (
	"runtime"
	"syscall/js"
	"weak"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvas2d"
)

func (s *surface) recordedCanvasImage(src driver.ImageSource, id driver.NodeID) *canvasImage {
	key := src.Key()
	if im := s.recordings[key]; im != nil {
		return im
	}
	r := src.Recording
	canvas := s.api.Call("createOffscreenCanvas", map[string]any{
		"type": "2d", "width": r.PixelWidth, "height": r.PixelHeight,
	})
	pending := false
	canvas2d.PaintRecording(canvas, r, canvasFont, func(src driver.ImageSource) (js.Value, float64, bool) {
		if src.Pixels != nil {
			return js.Undefined(), 1, false
		}
		im := s.canvasImage(src, id)
		pending = pending || (!im.loaded && !im.failed)
		return im.img, float64(src.Density()), im.loaded
	})
	if pending {
		return &canvasImage{}
	}
	im := &canvasImage{img: canvas, loaded: true}
	if s.recordings == nil {
		s.recordings = map[string]*canvasImage{}
	}
	s.recordings[key] = im
	owner := weak.Make(s)
	runtime.AddCleanup(r, func(key string) {
		s := owner.Value()
		if s == nil {
			return
		}
		var drop js.Func
		drop = js.FuncOf(func(js.Value, []js.Value) any {
			delete(s.recordings, key)
			drop.Release()
			return nil
		})
		js.Global().Call("setTimeout", drop, 0)
	}, key)
	return im
}
