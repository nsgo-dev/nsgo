package driver

import (
	"math"
	"strconv"
	"sync/atomic"
)

// CanvasRecording is an immutable drawing and raster size. The recording has
// its own identity so a later revision cannot reuse its native image.
type CanvasRecording struct {
	Ops         []DrawOp
	PixelWidth  int
	PixelHeight int
	Size        Size
	key         string
}

var canvasRecordingID atomic.Uint64

// NewCanvasRecording makes an image source for a standalone drawing. The
// caller hands over the ops and their image data as it would for ItemCanvas.
func NewCanvasRecording(ops []DrawOp, size Size, scale float32) ImageSource {
	w := math.Ceil(float64(size.Width) * float64(scale))
	h := math.Ceil(float64(size.Height) * float64(scale))
	// Match the largest backing-store area used by the canvas players.
	if !(w >= 1 && h >= 1 && w <= 16384 && h <= 16384 && w*h <= 1<<24) {
		return ImageSource{}
	}
	if !(size.Width > 0 && size.Height > 0 && scale > 0) {
		return ImageSource{}
	}
	return ImageSource{Size: size, Recording: &CanvasRecording{
		Ops: ops, PixelWidth: int(w), PixelHeight: int(h), Size: size,
		key: "\x00canvas:" + strconv.FormatUint(canvasRecordingID.Add(1), 10),
	}}
}
