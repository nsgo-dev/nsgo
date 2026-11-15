package driver

import (
	"cmp"
	"slices"
)

// Geometry is how a surface's display divides it: the regions an app can lay out in and the folds and hinges between them, in surface points, measured at the size it travels with (Engine.Resize, Tree.Resize). The zero value is the one-region form of a surface without a fold, which Divide returns for no obstructions.
type Geometry struct {
	// Regions are the usable regions, ordered by frame position: top to bottom, then left to right.
	Regions []Region
	// Obstructions are the folds and hinges on the surface, in the same order.
	Obstructions []Obstruction
}

// Region is a part of the surface an app can lay out in.
type Region struct {
	Frame Bounds
	// SafeArea is the part of each of the region's edges that system bars, notches and rounded corners cover: the surface's safe area where it falls in the region, which core fills in from the environment.
	SafeArea Edges
	// Reserved is the part of the region an obstruction that does not divide it takes, such as a fold whose pixels do not show, which the app must not place content across; empty when there is none.
	Reserved Bounds
}

// Obstruction is a fold or a hinge: its frame, its orientation and three properties that vary apart.
type Obstruction struct {
	Frame Bounds
	// Horizontal is an obstruction between a region above and one below; else it is vertical, between a region on the left and one on the right.
	Horizontal bool
	// Occluding says the pixels under it are not shown or touched.
	Occluding bool
	// Separating says the regions on either side read as logically apart.
	Separating bool
	State      HingeState
	// Angle is the hinge's angle in radians, where the platform reports one, as HasAngle says.
	Angle    float32
	HasAngle bool
}

// HingeState is how far a fold or hinge is open.
type HingeState uint8

const (
	HingeUnknown HingeState = iota
	HingeClosed
	HingeHalfOpen
	// HingeOpen is open as far as the device goes, which Android calls flat.
	HingeOpen
)

// Divide returns the geometry of a surface of size that obstructions lie on, in surface points: the obstructions within the surface, clipped to it, and the regions those that separate it and cross it from edge to edge leave on either side of their frames. An obstruction that does not divide the surface, such as one that occludes without separating, reserves its frame in the regions it lies in. Frames outside the surface, and those without length, are left out.
func Divide(size Size, obstructions []Obstruction) Geometry {
	var g Geometry
	for _, o := range obstructions {
		f, ok := clip(o.Frame, size)
		if !ok {
			continue
		}
		o.Frame = f
		g.Obstructions = append(g.Obstructions, o)
	}
	slices.SortStableFunc(g.Obstructions, func(a, b Obstruction) int {
		return cmp.Or(cmp.Compare(a.Frame.Origin.Y, b.Frame.Origin.Y), cmp.Compare(a.Frame.Origin.X, b.Frame.Origin.X))
	})
	var columns, rows [][2]Points
	columns = append(columns, [2]Points{0, size.Width})
	rows = append(rows, [2]Points{0, size.Height})
	var reserved []Bounds
	for _, o := range g.Obstructions {
		f := o.Frame
		switch {
		case o.Separating && !o.Horizontal && f.Origin.Y == 0 && f.Size.Height == size.Height:
			columns = cut(columns, f.Origin.X, f.Origin.X+f.Size.Width)
		case o.Separating && o.Horizontal && f.Origin.X == 0 && f.Size.Width == size.Width:
			rows = cut(rows, f.Origin.Y, f.Origin.Y+f.Size.Height)
		case f.Size.Width > 0 && f.Size.Height > 0:
			reserved = append(reserved, f)
		}
	}
	for _, r := range rows {
		for _, c := range columns {
			region := Region{Frame: Bounds{Origin: Point{X: c[0], Y: r[0]}, Size: Size{Width: c[1] - c[0], Height: r[1] - r[0]}}}
			for _, f := range reserved {
				if in, ok := intersect(f, region.Frame); ok {
					region.Reserved = union(region.Reserved, in)
				}
			}
			g.Regions = append(g.Regions, region)
		}
	}
	if len(g.Regions) == 0 {
		// Obstructions covering the whole surface leave the app the one region still.
		g.Regions = []Region{{Frame: Bounds{Size: size}}}
	}
	return g
}

// Equal reports whether two geometries have the same regions and obstructions, an empty list the same as none.
func (g Geometry) Equal(o Geometry) bool {
	return slices.Equal(g.Regions, o.Regions) && slices.Equal(g.Obstructions, o.Obstructions)
}

// Clone returns g with lists of its own.
func (g Geometry) Clone() Geometry {
	return Geometry{Regions: slices.Clone(g.Regions), Obstructions: slices.Clone(g.Obstructions)}
}

// cut removes [from, to] from the spans, splitting the one it falls in; a span left without length goes.
func cut(spans [][2]Points, from, to Points) [][2]Points {
	var out [][2]Points
	for _, s := range spans {
		if to < s[0] || from > s[1] {
			out = append(out, s)
			continue
		}
		if from > s[0] {
			out = append(out, [2]Points{s[0], from})
		}
		if to < s[1] {
			out = append(out, [2]Points{to, s[1]})
		}
	}
	return out
}

// clip returns f within a surface of size; ok is false for a frame outside it or without length.
func clip(f Bounds, size Size) (Bounds, bool) {
	x0, y0 := max(f.Origin.X, 0), max(f.Origin.Y, 0)
	x1, y1 := min(f.Origin.X+f.Size.Width, size.Width), min(f.Origin.Y+f.Size.Height, size.Height)
	if x1 < x0 || y1 < y0 || x1 == x0 && y1 == y0 {
		return Bounds{}, false
	}
	return Bounds{Origin: Point{X: x0, Y: y0}, Size: Size{Width: x1 - x0, Height: y1 - y0}}, true
}

// intersect returns the area a and b share; ok is false when they share none.
func intersect(a, b Bounds) (Bounds, bool) {
	x0, y0 := max(a.Origin.X, b.Origin.X), max(a.Origin.Y, b.Origin.Y)
	x1 := min(a.Origin.X+a.Size.Width, b.Origin.X+b.Size.Width)
	y1 := min(a.Origin.Y+a.Size.Height, b.Origin.Y+b.Size.Height)
	if x1 <= x0 || y1 <= y0 {
		return Bounds{}, false
	}
	return Bounds{Origin: Point{X: x0, Y: y0}, Size: Size{Width: x1 - x0, Height: y1 - y0}}, true
}

// union returns the smallest bounds holding a and b, a being empty for none.
func union(a, b Bounds) Bounds {
	if a.Size.Width <= 0 || a.Size.Height <= 0 {
		return b
	}
	x0, y0 := min(a.Origin.X, b.Origin.X), min(a.Origin.Y, b.Origin.Y)
	x1 := max(a.Origin.X+a.Size.Width, b.Origin.X+b.Size.Width)
	y1 := max(a.Origin.Y+a.Size.Height, b.Origin.Y+b.Size.Height)
	return Bounds{Origin: Point{X: x0, Y: y0}, Size: Size{Width: x1 - x0, Height: y1 - y0}}
}
