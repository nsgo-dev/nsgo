package driver

import (
	"math"
	"testing"
)

func TestUnprojectFindsTheDrawnPlane(t *testing.T) {
	for _, tr := range []Transform{
		{TranslateY: -100}, {ScaleX: -2, ScaleY: .5, Rotate: 37},
		{RotateX: 30, RotateY: -25, Perspective: 300}, {SkewX: 28},
	} {
		m := tr.Matrix(Point{X: 40, Y: 30})
		x, y := m.Apply(13, 29)
		p, ok := m.Unproject(Point{X: Points(x), Y: Points(y)})
		if !ok || math.Abs(float64(p.X)-13) > .001 || math.Abs(float64(p.Y)-29) > .001 {
			t.Errorf("%+v: unproject = %+v, %v", tr, p, ok)
		}
	}
	for _, m := range []Matrix3D{{}, {1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1},
		Transform{RotateY: 90}.Matrix(Point{}), Translate3D(math.Inf(1), 0, 0)} {
		if p, ok := m.Unproject(Point{X: 1, Y: 2}); ok {
			t.Errorf("collapsed or non-finite plane %v hit at %+v", m, p)
		}
	}
}

func TestReachTransformedChildrenOutsideTheLayoutBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode PointerEvents
		clip bool
		want bool
	}{
		{"auto", PointerEventsAuto, false, true},
		{"box-none", PointerEventsBoxNone, false, true},
		{"box-only", PointerEventsBoxOnly, false, false},
		{"none", PointerEventsNone, false, false},
		{"clipped", PointerEventsAuto, true, false},
		{"clipped-box-none", PointerEventsBoxNone, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			self, children := ReachView(Size{Width: 40, Height: 40}, &ViewProps{PointerEvents: tc.mode}, tc.clip, Point{X: 10, Y: -90})
			if self || children != tc.want {
				t.Fatalf("reach outside bounds = (%v, %v), want (false, %v)", self, children, tc.want)
			}
		})
	}
}
