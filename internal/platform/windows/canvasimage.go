//go:build windows

package windows

import (
	"runtime"
	"unsafe"
	"weak"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func (p *painter) canvasSource(src driver.ImageSource, v *view) *imageSlot {
	if src.Recording == nil {
		return p.w.canvasImage(src)
	}
	w, key := p.w, src.Key()
	slot := w.canvasRecordings[key]
	if slot != nil && slot.generation == gfxGeneration {
		return slot
	}
	r := src.Recording
	bitmap, pending := p.makeCanvasImage(v, r)
	if pending {
		return nil
	}
	if slot == nil {
		if w.canvasRecordings == nil {
			w.canvasRecordings = map[string]*imageSlot{}
		}
		slot = &imageSlot{src: driver.ImageSource{Size: r.Size}}
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
	}
	releaseAll(&slot.bitmap)
	slot.bitmap, slot.generation = bitmap, gfxGeneration
	slot.pic = nil
	if bitmap != nil {
		slot.pic = &picture{width: r.PixelWidth, height: r.PixelHeight}
	}
	return slot
}

func (p *painter) makeCanvasImage(v *view, r *driver.CanvasRecording) (*comObject, bool) {
	// A separate context lets nested recordings and the outer canvas each
	// keep their target and clip stack.
	var dc *comObject
	if failed(p.g.device.call(d2dDeviceCreateDeviceContext, 0, uintptr(unsafe.Pointer(&dc)))) {
		return nil, false
	}
	defer dc.release()
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied,
		dpiX: 96, dpiY: 96, options: d2dBitmapTarget}
	var bitmap *comObject
	if failed(dc.callSize(dcCreateBitmap, uint32(r.PixelWidth), uint32(r.PixelHeight), 0, 0,
		uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&bitmap)))) {
		return nil, false
	}
	w, h := float32(r.Size.Width), float32(r.Size.Height)
	sx, sy := float32(r.PixelWidth)/w, float32(r.PixelHeight)/h
	dc.call(dcSetTarget, bitmap.ptr())
	dc.callF(rtSetDpi, 0b11, f32(96*sx), f32(96*sy))
	dc.call(rtBeginDraw)
	clear := colorF{}
	dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
	render := &painter{w: p.w, g: p.g, dc: dc, m: identity, scale: max(sx, sy), alpha: 1, brush: p.brush}
	c := render.newCanvasPlayer(v, identity, w, h)
	c.off = bitmap
	c.playAll(r.Ops)
	hr := dc.call(rtEndDraw, 0, 0)
	dc.call(dcSetTarget, 0)
	if c.waited || failed(hr) {
		bitmap.release()
		return nil, c.waited
	}
	return bitmap, false
}
