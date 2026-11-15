package android

import "github.com/nsgo-dev/nsgo/internal/driver"

// The FoldingFeature constants of androidx.window.extensions, which the runtime passes on as the library reports them.
const (
	foldTypeFold  = 1
	foldTypeHinge = 2

	foldStateFlat       = 1
	foldStateHalfOpened = 2
)

// fold is a FoldingFeature the runtime reported with a size: its frame in the view's points, which the runtime clips to the view in pixels so an edge at the view's is exactly the size's, its type and its state.
type fold struct {
	frame       driver.Bounds
	kind, state byte
}

// readSize reads the size msgStart and msgResize carry and the folds measured at it.
func readSize(r *reader) (driver.Size, []fold) {
	size := driver.Size{Width: r.points(), Height: r.points()}
	n := r.u32()
	var folds []fold
	for i := uint32(0); i < n && len(r.buf) > 0; i++ {
		left, top, right, bottom := r.points(), r.points(), r.points(), r.points()
		f := fold{frame: driver.Bounds{Origin: driver.Point{X: left, Y: top}, Size: driver.Size{Width: right - left, Height: bottom - top}}}
		f.kind, f.state = r.byte(), r.byte()
		folds = append(folds, f)
	}
	return size, folds
}

// foldGeometry is the geometry folds give a view of size. It derives the properties as Jetpack's HardwareFoldingFeature does: occluding when the bounds have area, separating for a hinge or a half-open fold, horizontal when wider than tall; and keeps only what Jetpack keeps of the library's report, a feature of a known type and state that crosses the view from edge to edge without covering it, so one measured at another size of the window is dropped.
func foldGeometry(size driver.Size, folds []fold) driver.Geometry {
	var obstructions []driver.Obstruction
	for _, f := range folds {
		var state driver.HingeState
		switch f.state {
		case foldStateFlat:
			state = driver.HingeOpen
		case foldStateHalfOpened:
			state = driver.HingeHalfOpen
		default:
			continue
		}
		if f.kind != foldTypeFold && f.kind != foldTypeHinge {
			continue
		}
		b := f.frame
		x0, y0 := max(b.Origin.X, 0), max(b.Origin.Y, 0)
		x1, y1 := min(b.Origin.X+b.Size.Width, size.Width), min(b.Origin.Y+b.Size.Height, size.Height)
		across, down := x0 == 0 && x1 == size.Width, y0 == 0 && y1 == size.Height
		if across == down || x1 < x0 || y1 < y0 {
			continue
		}
		obstructions = append(obstructions, driver.Obstruction{
			Frame:      b,
			Horizontal: b.Size.Width > b.Size.Height,
			Occluding:  b.Size.Width > 0 && b.Size.Height > 0,
			Separating: f.kind == foldTypeHinge || f.state == foldStateHalfOpened,
			State:      state,
		})
	}
	return driver.Divide(size, obstructions)
}
