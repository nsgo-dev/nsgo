package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// RecordCanvasImage records an independent drawing into an image source for
// Canvas. Size is in points and scale is pixels per point (zero uses 1). The
// callback runs once with the scope's text engine and the platform renders
// the recorded arguments. Keep the source to draw it in later frames. Record
// a new source when its contents change. An invalid size or scale returns
// an empty source.
func RecordCanvasImage(cx *Scope, size Size, scale float32, draw func(*Drawing)) ImageSource {
	if scale == 0 {
		scale = 1
	}
	src := driver.NewCanvasRecording(nil, size, scale)
	if src.Empty() {
		return src
	}
	t := cx.tree()
	var text driver.TextEngine
	if t != nil {
		text = t.text
	}
	d := driver.NewDrawing(text)
	if draw != nil {
		cx.Untrack(func() { draw(d) })
	}
	pickDrawingScales(d, scale)
	var recorder canvasNode
	src.Recording.Ops = recorder.withFrames(d).Ops
	return src
}
