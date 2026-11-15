//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// regionKinds are UIViewReservedRegionKind's occlusion and division kinds, from iOS 27.1; zero before it.
var regionKinds struct {
	loaded              bool
	occlusion, division objc.ID
}

// reservedRegions returns the surface view's active reserved regions in its coordinates, as layoutSubviews reports its size; none on a system without them.
func (s *surface) reservedRegions() []reservedRegion {
	if !regionKinds.loaded {
		regionKinds.loaded = true
		if c := apple.Class("UIViewReservedRegionKind"); c != 0 {
			regionKinds.occlusion = objc.ID(c).Send(sel("occlusionRegionKind"))
			regionKinds.division = objc.ID(c).Send(sel("divisionRegionKind"))
		}
	}
	if regionKinds.occlusion == 0 || !objc.Send[bool](s.native, sel("respondsToSelector:"), sel("reservedRegionsOfKind:")) {
		return nil
	}
	var out []reservedRegion
	for _, kind := range []struct {
		id       objc.ID
		division bool
	}{{regionKinds.occlusion, false}, {regionKinds.division, true}} {
		list := s.native.Send(sel("reservedRegionsOfKind:"), kind.id)
		for i := range objc.Send[uint](list, sel("count")) {
			r := list.Send(sel("objectAtIndex:"), i)
			if !objc.Send[bool](r, sel("isActive")) {
				continue
			}
			f := objc.Send[apple.CGRect](r, sel("frame"))
			out = append(out, reservedRegion{division: kind.division, frame: driver.Bounds{
				Origin: driver.Point{X: driver.Points(f.Origin.X), Y: driver.Points(f.Origin.Y)},
				Size:   driver.Size{Width: driver.Points(f.Size.Width), Height: driver.Points(f.Size.Height)},
			}})
		}
	}
	return out
}

// watchHinge adds a UIHingeInteraction to the surface view on iOS 27.1: each update keeps the hinge's status and angle and lays the view out again, so a change of state alone is a resize to the same size with the new geometry.
func (s *surface) watchHinge() {
	c := apple.Class("UIHingeInteraction")
	if c == 0 {
		return
	}
	handler := objc.NewBlock(func(_ objc.Block, _, update objc.ID) {
		h := update.Send(sel("hinge"))
		if h == 0 {
			s.hinge = hingeReading{}
		} else {
			s.hinge = hingeReading{known: true, status: objc.Send[int](h, sel("status")), angle: objc.Send[float64](h, sel("angle"))}
		}
		s.native.Send(sel("setNeedsLayout"))
	})
	interaction := objc.ID(c).Send(selAlloc).Send(sel("initWithUpdateHandler:"), handler)
	handler.Release()
	s.native.Send(sel("addInteraction:"), interaction)
	interaction.Send(selRelease)
}

// verticalBarEdge is the edge the system chose for a vertical bar, from the trait collection on iOS 27.1.
func verticalBarEdge(traits objc.ID) driver.VerticalBarEdge {
	if !objc.Send[bool](traits, sel("respondsToSelector:"), sel("verticalBarEdge")) {
		return driver.VerticalBarUnspecified
	}
	const leading, trailing = 1, 2
	switch objc.Send[int](traits, sel("verticalBarEdge")) {
	case leading:
		return driver.VerticalBarLeading
	case trailing:
		return driver.VerticalBarTrailing
	}
	return driver.VerticalBarUnspecified
}
