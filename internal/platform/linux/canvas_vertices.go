//go:build linux && !android

package linux

import (
	"math"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvasmesh"
)

func (p *canvasPlayer) vertices(op *driver.DrawOp) {
	if op.Image == nil {
		return
	}
	if draw, ok := op.ImageRect(); ok {
		p.image(&draw)
		return
	}
	var texture canvasmesh.Texture
	if src := *op.Image; src.Pixels != nil {
		texture = canvasmesh.RGBA(src.Pixels, src.Size)
	} else {
		slot := p.w.canvasImage(src, p.v)
		if slot == nil || slot.pic == nil {
			return
		}
		pic := slot.pic
		texture = canvasmesh.Texture{Pix: pic.pix, Width: pic.width, Height: pic.height,
			Stride: pic.width * 4, Points: natural(slot), BGRA: true}
	}
	var m cairoMatrix
	cairoGetMatrix(p.cr, &m)
	pix := canvasmesh.Raster(op, texture, max(math.Hypot(m.xx, m.yx), math.Hypot(m.xy, m.yy)))
	if pix == nil {
		return
	}
	var slot imageSlot
	slot.showPixels(driver.ImageSource{Pixels: pix})
	defer slot.reset(driver.ImageSource{})
	p.drawImage(op, &slot)
}
