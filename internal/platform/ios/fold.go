package ios

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

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

// regionHold is how long after the surface view changes size its reserved regions go unread: during a rotation or a move to the other display, reservedRegionsOfKind: can answer for a layout pass or more in the previous coordinate space, and nothing tells when it stops, so the regions are read again once it has passed and later still (regionReads).
const regionHold = 250 * time.Millisecond

// regionReads are when, after a size change, the surface view is laid out again so its reserved regions are read at the new size, as UIKit tells nothing when they change.
var regionReads = []time.Duration{regionHold, 3 * regionHold, 8 * regionHold}

// regionClock follows the sizes the surface view's reserved regions are read at.
type regionClock struct {
	size  driver.Size
	since time.Time
}

// read reports whether regions read at size now can be used: not while regionHold has not passed since the size changed, the size the surface first reads at excepted. changed is true when size is a new one, for the caller to read again later.
func (c *regionClock) read(size driver.Size, now time.Time) (trusted, changed bool) {
	switch {
	case c.since.IsZero():
		c.size, c.since = size, now.Add(-regionHold)
	case size != c.size:
		c.size, c.since = size, now
		changed = true
	}
	return now.Sub(c.since) >= regionHold, changed
}

// regionsFit reports whether every region lies within a view of size, as one measured at that size does: a frame past its bounds, such as the inner display's division at the outer display's size, was measured in another coordinate space.
func regionsFit(size driver.Size, regions []reservedRegion) bool {
	const slack = 0.5
	for _, r := range regions {
		f := r.frame
		if f.Origin.X < -slack || f.Origin.Y < -slack || f.Origin.X+f.Size.Width > size.Width+slack || f.Origin.Y+f.Size.Height > size.Height+slack {
			return false
		}
	}
	return true
}
