package ios

import (
	"reflect"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func bounds(x, y, w, h driver.Points) driver.Bounds {
	return driver.Bounds{Origin: driver.Point{X: x, Y: y}, Size: driver.Size{Width: w, Height: h}}
}

// TestFoldGeometry maps UIKit's observations recorded as the backend reads them: the surface view's active reserved regions and the hinge interaction's latest update.
func TestFoldGeometry(t *testing.T) {
	size := driver.Size{Width: 900, Height: 640}
	hinge := []reservedRegion{{frame: bounds(440, 0, 20, 640)}, {frame: bounds(440, 0, 20, 640), division: true}}
	cases := []struct {
		name    string
		size    driver.Size
		regions []reservedRegion
		hinge   hingeReading
		want    driver.Geometry
	}{
		{"an open hinge", size, hinge, hingeReading{true, hingeStatusFullyOpen, 3.14}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 440, 640)}, {Frame: bounds(460, 0, 440, 640)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(440, 0, 20, 640), Occluding: true, Separating: true, State: driver.HingeOpen, Angle: 3.14, HasAngle: true}},
			State:        driver.HingeOpen, Angle: 3.14, HasAngle: true,
		}},
		{"a half-open hinge", size, hinge, hingeReading{true, hingeStatusPartiallyOpen, 1.5}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 440, 640)}, {Frame: bounds(460, 0, 440, 640)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(440, 0, 20, 640), Occluding: true, Separating: true, State: driver.HingeHalfOpen, Angle: 1.5, HasAngle: true}},
			State:        driver.HingeHalfOpen, Angle: 1.5, HasAngle: true,
		}},
		{"a closed hinge", size, hinge, hingeReading{true, hingeStatusClosed, 0}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 440, 640)}, {Frame: bounds(460, 0, 440, 640)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(440, 0, 20, 640), Occluding: true, Separating: true, State: driver.HingeClosed, HasAngle: true}},
			State:        driver.HingeClosed, HasAngle: true,
		}},
		{"a division alone, before the hinge reports", size, []reservedRegion{{frame: bounds(450, 0, 0, 640), division: true}}, hingeReading{}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 450, 640)}, {Frame: bounds(450, 0, 450, 640)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(450, 0, 0, 640), Separating: true}},
		}},
		{"rotated", driver.Size{Width: 640, Height: 900}, []reservedRegion{{frame: bounds(0, 440, 640, 20)}, {frame: bounds(0, 440, 640, 20), division: true}}, hingeReading{true, hingeStatusPartiallyOpen, 1.5}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 640, 440)}, {Frame: bounds(0, 460, 640, 440)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(0, 440, 640, 20), Horizontal: true, Occluding: true, Separating: true, State: driver.HingeHalfOpen, Angle: 1.5, HasAngle: true}},
			State:        driver.HingeHalfOpen, Angle: 1.5, HasAngle: true,
		}},
		{"a vertical bar on the leading edge", size, append([]reservedRegion{{frame: bounds(0, 0, 60, 640)}}, hinge...), hingeReading{true, hingeStatusFullyOpen, 3.14}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 440, 640)}, {Frame: bounds(460, 0, 440, 640)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(440, 0, 20, 640), Occluding: true, Separating: true, State: driver.HingeOpen, Angle: 3.14, HasAngle: true}},
			State:        driver.HingeOpen, Angle: 3.14, HasAngle: true,
		}},
		{"a vertical bar on the trailing edge", size, append([]reservedRegion{{frame: bounds(840, 0, 60, 640)}}, hinge...), hingeReading{true, hingeStatusFullyOpen, 3.14}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 440, 640)}, {Frame: bounds(460, 0, 440, 640)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(440, 0, 20, 640), Occluding: true, Separating: true, State: driver.HingeOpen, Angle: 3.14, HasAngle: true}},
			State:        driver.HingeOpen, Angle: 3.14, HasAngle: true,
		}},
		{"an older system without the selectors", size, nil, hingeReading{}, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 900, 640)}},
		}},
		{"open flat without a region", driver.Size{Width: 669, Height: 951}, []reservedRegion{{frame: bounds(525, 0, 144, 82)}}, hingeReading{true, hingeStatusFullyOpen, 3.141592653589793}, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 669, 951)}},
			State:   driver.HingeOpen, Angle: 3.141592653589793, HasAngle: true,
		}},
		{"closed without a region", driver.Size{Width: 466, Height: 678}, nil, hingeReading{true, hingeStatusClosed, 0}, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 466, 678)}},
			State:   driver.HingeClosed, HasAngle: true,
		}},
		{"a status the system adds later", size, nil, hingeReading{true, 9, 1}, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 900, 640)}},
			Angle:   1, HasAngle: true,
		}},
		{"the hinge left its hierarchy", size, hinge, hingeReading{}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 440, 640)}, {Frame: bounds(460, 0, 440, 640)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(440, 0, 20, 640), Occluding: true, Separating: true}},
		}},
	}
	for _, c := range cases {
		if got := foldGeometry(c.size, driver.Edges{}, c.regions, c.hinge); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}

// TestFoldGeometryCutouts leaves to the safe area the occlusion regions the iPhone Duo simulator (iOS 27.2) reports for the outer display's camera and the vertical bar around it, which lie within the safe area's insets.
func TestFoldGeometryCutouts(t *testing.T) {
	closed := hingeReading{true, hingeStatusClosed, 0}
	cases := []struct {
		name    string
		size    driver.Size
		safe    driver.Edges
		regions []reservedRegion
		want    driver.Geometry
	}{
		{"the camera inside the trailing bar", driver.Size{Width: 466, Height: 678}, driver.Edges{Right: 84, Bottom: 34},
			[]reservedRegion{{frame: bounds(399.6666666666667, 29.333333333333332, 37, 37)}, {frame: bounds(382, 0, 84, 174)}},
			driver.Geometry{Regions: []driver.Region{{Frame: bounds(0, 0, 466, 678)}}, State: driver.HingeClosed, HasAngle: true}},
		{"the camera inside the trailing bar in landscape", driver.Size{Width: 678, Height: 466}, driver.Edges{Right: 84, Bottom: 34},
			[]reservedRegion{{frame: bounds(611.6666666666666, 399.6666666666667, 37, 37)}, {frame: bounds(594, 384, 84, 82)}},
			driver.Geometry{Regions: []driver.Region{{Frame: bounds(0, 0, 678, 466)}}, State: driver.HingeClosed, HasAngle: true}},
		{"the camera inside the leading bar", driver.Size{Width: 678, Height: 466}, driver.Edges{Bottom: 34, Left: 84},
			[]reservedRegion{{frame: bounds(29.333333333333332, 29.333333333333314, 37, 37)}, {frame: bounds(0, 0, 84, 82)}},
			driver.Geometry{Regions: []driver.Region{{Frame: bounds(0, 0, 678, 466)}}, State: driver.HingeClosed, HasAngle: true}},
		{"an occlusion reaching into the safe area", driver.Size{Width: 466, Height: 678}, driver.Edges{Right: 84, Bottom: 34},
			[]reservedRegion{{frame: bounds(360, 300, 37, 37)}},
			driver.Geometry{
				Regions:      []driver.Region{{Frame: bounds(0, 0, 466, 678), Reserved: bounds(360, 300, 37, 37)}},
				Obstructions: []driver.Obstruction{{Frame: bounds(360, 300, 37, 37), Occluding: true, State: driver.HingeClosed, HasAngle: true}},
				State:        driver.HingeClosed, HasAngle: true,
			}},
	}
	for _, c := range cases {
		if got := foldGeometry(c.size, c.safe, c.regions, closed); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}

// TestStaleRegions leaves out what the iPhone Duo simulator (iOS 27.2) answered in the previous display's coordinate space as it moved between displays: the inner display's division at the outer display's size, and anything read just after a size change.
func TestStaleRegions(t *testing.T) {
	outer := driver.Size{Width: 678, Height: 466}
	if regionsFit(outer, []reservedRegion{{frame: bounds(182.5, 0, 40, 669), division: true}}) {
		t.Error("the inner display's division fits the outer display")
	}
	if !regionsFit(outer, []reservedRegion{{frame: bounds(611.6666666666666, 399.6666666666667, 37, 37)}, {frame: bounds(594, 384, 84, 82)}}) {
		t.Error("the outer display's own regions do not fit it")
	}
	if !regionsFit(driver.Size{Width: 900, Height: 640}, []reservedRegion{{frame: bounds(450, 0, 0, 640.25), division: true}}) {
		t.Error("a division rounded past the edge does not fit")
	}
	// Once the hold has passed after closing, the vertical bar is still sliding in: read at 6 ms and at 136 ms, it lies along the edge at either frame and the camera within the safe area's inset, so the outer display has one region throughout.
	for _, bar := range []driver.Bounds{bounds(596, 266, 82, 200), bounds(596, 382, 82, 84)} {
		regions := []reservedRegion{{frame: bounds(611.6666666666666, 399.6666666666667, 37, 37)}, {frame: bar}}
		g := foldGeometry(outer, driver.Edges{Right: 84, Bottom: 34}, regions, hingeReading{true, hingeStatusClosed, 0})
		if !regionsFit(outer, regions) || len(g.Regions) != 1 || len(g.Obstructions) != 0 {
			t.Errorf("the bar at %v: %+v, want one region", bar, g)
		}
	}

	var c regionClock
	start := time.Unix(1000, 0)
	inner := driver.Size{Width: 951, Height: 669}
	steps := []struct {
		size             driver.Size
		at               time.Duration
		trusted, changed bool
	}{
		{outer, 0, true, false},               // the size the surface starts at
		{outer, time.Second, true, false},     // a hinge update at the same size
		{inner, 2 * time.Second, false, true}, // opening: the previous display's frames come first
		{inner, 2*time.Second + 10*time.Millisecond, false, false},
		{inner, 2*time.Second + regionHold, true, false}, // read again once the hold has passed
		{outer, 5 * time.Second, false, true},            // closing
		{outer, 5*time.Second + regionHold - 1, false, false},
		{outer, 6 * time.Second, true, false},
	}
	for i, st := range steps {
		trusted, changed := c.read(st.size, start.Add(st.at))
		if trusted != st.trusted || changed != st.changed {
			t.Errorf("step %d at %v: trusted %v changed %v, want %v %v", i, st.at, trusted, changed, st.trusted, st.changed)
		}
	}
}
