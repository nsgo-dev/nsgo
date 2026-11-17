//go:build darwin && !ios

package macos

import (
	"math"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// pointerMatrix is the transform v's layer shows now, around its top left, with its parent's perspective.
func (v *view) pointerMatrix() driver.Matrix3D {
	if !v.transformed {
		// A parent's perspective leaves an untransformed plane where it is.
		return driver.Identity3D
	}
	m := apple.PointerMatrix(v.native.Send(selLayer))
	if p := v.up; p != nil {
		f := v.nativeFrame()
		m = driver.ChildPerspective(p.style.ChildPerspective, p.frame.Size, f.Origin).Mul(m)
	}
	return m
}

// windowPoint undoes the transforms AppKit's view conversion leaves
// out. Converting between a view and its container still supplies the
// native scroll bounds origin, holder frame and flipped coordinates.
func (v *view) windowPoint(at apple.CGPoint) apple.CGPoint {
	var p apple.CGPoint
	if parent := v.up; parent != nil {
		p = parent.windowPoint(at)
		p = objc.Send[apple.CGPoint](v.native.Send(selSuperview), selConvertPoint, p, parent.native)
		f := v.nativeFrame()
		p.X -= float64(f.Origin.X)
		p.Y -= float64(f.Origin.Y)
	} else {
		p = objc.Send[apple.CGPoint](v.native, selConvertPoint, at, objc.ID(0))
	}
	local, ok := v.pointerMatrix().Unproject(driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)})
	if !ok {
		return apple.CGPoint{X: math.Inf(1), Y: math.Inf(1)}
	}
	return apple.CGPoint{X: float64(local.X), Y: float64(local.Y)}
}
