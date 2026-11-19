package ios

import "github.com/nsgo-dev/nsgo/internal/driver"

// This file builds on every system, so tests check the mapping from UIKit's observations on any Mac.

// reservedRegion is an active UIViewReservedRegion of the surface view: its frame in the view's points, which includes its margins for interactive content, and whether it is of divisionRegionKind rather than occlusionRegionKind.
type reservedRegion struct {
	frame    driver.Bounds
	division bool
}

// hingeReading is the latest UIHinge a UIHingeInteraction reported; known is false before one, and once the interaction left a hierarchy that provides hinge updates.
type hingeReading struct {
	known  bool
	status int
	angle  float64
}

// UIHingeStatus's values.
const (
	hingeStatusUnknown       = 0
	hingeStatusClosed        = 1
	hingeStatusPartiallyOpen = 2
	hingeStatusFullyOpen     = 3
)

// foldGeometry is the geometry of a surface view of size with safe area safe, regions and the hinge's latest reading: an occlusion region becomes an occluding obstruction and a division region a separating one, one of each at the same frame a single obstruction both occluding and separating; and the hinge's status and angle are the geometry's and every obstruction's, with or without obstructions. An occlusion region that divides nothing is left to the safe area, as the system's bars and cutouts are, when it lies along an edge of the view, so the vertical bar the system may place on either edge never becomes an obstruction, or wholly within the safe area's insets, as a camera beside the vertical bar does.
func foldGeometry(size driver.Size, safe driver.Edges, regions []reservedRegion, h hingeReading) driver.Geometry {
	inside := driver.Bounds{Origin: driver.Point{X: safe.Left, Y: safe.Top}, Size: driver.Size{Width: size.Width - safe.Left - safe.Right, Height: size.Height - safe.Top - safe.Bottom}}
	var obstructions []driver.Obstruction
	for _, r := range regions {
		i := -1
		for j, o := range obstructions {
			if o.Frame == r.frame {
				i = j
			}
		}
		if i < 0 {
			obstructions = append(obstructions, driver.Obstruction{Frame: r.frame, Horizontal: r.frame.Size.Width > r.frame.Size.Height})
			i = len(obstructions) - 1
		}
		if r.division {
			obstructions[i].Separating = true
		} else {
			obstructions[i].Occluding = true
		}
	}
	var state driver.HingeState
	switch {
	case !h.known:
	case h.status == hingeStatusClosed:
		state = driver.HingeClosed
	case h.status == hingeStatusPartiallyOpen:
		state = driver.HingeHalfOpen
	case h.status == hingeStatusFullyOpen:
		state = driver.HingeOpen
	}
	var angle float32
	if h.known {
		angle = float32(h.angle)
	}
	kept := obstructions[:0]
	for _, o := range obstructions {
		f := o.Frame
		edge := !o.Horizontal && (f.Origin.X <= 0 || f.Origin.X+f.Size.Width >= size.Width) ||
			o.Horizontal && (f.Origin.Y <= 0 || f.Origin.Y+f.Size.Height >= size.Height)
		if !o.Separating && (edge || !overlaps(f, inside)) {
			continue
		}
		o.State, o.Angle, o.HasAngle = state, angle, h.known
		kept = append(kept, o)
	}
	g := driver.Divide(size, kept)
	g.State, g.Angle, g.HasAngle = state, angle, h.known
	return g
}

// overlaps reports whether a and b share an area.
func overlaps(a, b driver.Bounds) bool {
	return a.Origin.X < b.Origin.X+b.Size.Width && b.Origin.X < a.Origin.X+a.Size.Width &&
		a.Origin.Y < b.Origin.Y+b.Size.Height && b.Origin.Y < a.Origin.Y+a.Size.Height
}
