package android

import (
	"runtime"
	"weak"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func (s *surface) sendCanvasRecording(surface uint32, key string, request uint32, r *driver.CanvasRecording) {
	if err := s.send(msgCanvasImage, func(w *writer) {
		w.u32(surface)
		w.str(key)
		w.u32(request)
		w.byte(2)
		w.u32(uint32(r.PixelWidth))
		w.u32(uint32(r.PixelHeight))
		w.points(r.Size.Width)
		w.points(r.Size.Height)
		writeDrawing(w, &driver.Drawing{Ops: r.Ops})
	}); err != nil {
		s.sendCanvasImage(surface, key, request, decodedImage{}, false)
		return
	}
	owner := weak.Make(s)
	runtime.AddCleanup(r, func(key string) {
		if s := owner.Value(); s != nil {
			_ = s.send(msgCanvasImage, func(w *writer) {
				w.u32(surface)
				w.str(key)
				w.u32(0)
				w.byte(3)
			})
		}
	}, key)
}
