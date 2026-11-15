package android

import (
	"reflect"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func bounds(x, y, w, h driver.Points) driver.Bounds {
	return driver.Bounds{Origin: driver.Point{X: x, Y: y}, Size: driver.Size{Width: w, Height: h}}
}

// TestFoldGeometry maps WindowManager observations recorded as the runtime reports them: FoldingFeature bounds in the view's points, with the type and state.
func TestFoldGeometry(t *testing.T) {
	open := driver.Size{Width: 841, Height: 701}
	cases := []struct {
		name  string
		size  driver.Size
		folds []fold
		want  driver.Geometry
	}{
		{"an open fold", open, []fold{{bounds(420.5, 0, 0, 701), foldTypeFold, foldStateFlat}}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 841, 701)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(420.5, 0, 0, 701), State: driver.HingeOpen}},
		}},
		{"a half-open fold", open, []fold{{bounds(420.5, 0, 0, 701), foldTypeFold, foldStateHalfOpened}}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 420.5, 701)}, {Frame: bounds(420.5, 0, 420.5, 701)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(420.5, 0, 0, 701), Separating: true, State: driver.HingeHalfOpen}},
		}},
		{"an open hinge between two panels", driver.Size{Width: 1114, Height: 705}, []fold{{bounds(540, 0, 34, 705), foldTypeHinge, foldStateFlat}}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 540, 705)}, {Frame: bounds(574, 0, 540, 705)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(540, 0, 34, 705), Occluding: true, Separating: true, State: driver.HingeOpen}},
		}},
		{"rotated, half open on a table", driver.Size{Width: 701, Height: 841}, []fold{{bounds(0, 420.5, 701, 0), foldTypeFold, foldStateHalfOpened}}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 701, 420.5)}, {Frame: bounds(0, 420.5, 701, 420.5)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(0, 420.5, 701, 0), Horizontal: true, Separating: true, State: driver.HingeHalfOpen}},
		}},
		{"in a split screen beside the fold", driver.Size{Width: 420, Height: 701}, nil, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 420, 701)}},
		}},
		{"a window on the other side of the fold, which lies past its left", driver.Size{Width: 420, Height: 701}, []fold{{bounds(-0.5, 0, 0, 701), foldTypeFold, foldStateHalfOpened}}, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 420, 701)}},
		}},
		{"a view embedded in part of the window, which the fold crosses past its edges", driver.Size{Width: 600, Height: 400}, []fold{{bounds(300, -100, 0, 701), foldTypeFold, foldStateHalfOpened}}, driver.Geometry{
			Regions:      []driver.Region{{Frame: bounds(0, 0, 300, 400)}, {Frame: bounds(300, 0, 300, 400)}},
			Obstructions: []driver.Obstruction{{Frame: bounds(300, 0, 0, 400), Separating: true, State: driver.HingeHalfOpen}},
		}},
		{"a feature that crosses the view neither way", open, []fold{{bounds(420.5, 100, 0, 300), foldTypeFold, foldStateHalfOpened}}, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 841, 701)}},
		}},
		{"a feature of an unknown type or state", open, []fold{{bounds(420.5, 0, 0, 701), 3, foldStateFlat}, {bounds(420.5, 0, 0, 701), foldTypeFold, 3}}, driver.Geometry{
			Regions: []driver.Region{{Frame: bounds(0, 0, 841, 701)}},
		}},
	}
	for _, c := range cases {
		if got := foldGeometry(c.size, c.folds); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}

func TestReadSize(t *testing.T) {
	// As NsgoView.sendSize writes it: the size, the count, then each fold's left, top, right, bottom, type and state.
	var w writer
	for _, f := range []float32{841, 701} {
		w.float(f)
	}
	w.u32(1)
	for _, f := range []float32{540, 0, 574, 701} {
		w.float(f)
	}
	w.byte(foldTypeHinge)
	w.byte(foldStateFlat)
	size, folds := readSize(&reader{w.buf})
	if size != (driver.Size{Width: 841, Height: 701}) || !reflect.DeepEqual(folds, []fold{{bounds(540, 0, 34, 701), foldTypeHinge, foldStateFlat}}) {
		t.Errorf("read %v %+v", size, folds)
	}
	// A size from before the folds were sent reads as none.
	w = writer{}
	w.float(841)
	w.float(701)
	if size, folds := readSize(&reader{w.buf}); size.Width != 841 || folds != nil {
		t.Errorf("read %v %+v without folds", size, folds)
	}
}
