package driver

import "testing"

func TestTouchGeometryRetainsTheResponderPathAndSnapshotsEachReport(t *testing.T) {
	var tracker TouchTracker
	shift := Points(0)
	mask := EventMask(1 << EventTouch)
	hit := func(p Point) NodeID {
		if p.X < 100 {
			return 11
		}
		return 20
	}
	parent := func(id NodeID) (NodeID, EventMask) {
		switch id {
		case 11:
			return 10, 0
		case 10, 20:
			return 1, mask
		default:
			return 0, mask
		}
	}
	position := func(id NodeID, p Point) (Point, bool) {
		if id == 10 {
			p.X = (p.X - shift) / 2
		}
		return p, true
	}
	first := Event{Phase: GestureBegan, Touches: []Touch{{ID: 7, PagePosition: Point{X: 20, Y: 30}, Changed: true}}}
	tracker.Capture(&first, hit, parent, position)
	if first.TouchGeometry.Targets[7] != 11 || first.TouchGeometry.Positions[10][0] != (Point{X: 10, Y: 30}) {
		t.Fatalf("initial geometry = %+v", first.TouchGeometry)
	}
	shift = 40 // the platform moved the bound translation between events
	second := Event{Phase: GestureBegan, Touches: []Touch{
		{ID: 7, PagePosition: Point{X: 60, Y: 30}},
		{ID: 9, PagePosition: Point{X: 140, Y: 50}, Changed: true},
	}}
	tracker.Capture(&second, hit, parent, position)
	if second.TouchGeometry.Positions[10][0].X != 10 || second.TouchGeometry.Positions[10][1].X != 50 {
		t.Fatalf("two-finger positions = %v", second.TouchGeometry.Positions[10])
	}
	if first.TouchGeometry.Positions[10][0].X != 10 || len(first.TouchGeometry.Positions[10]) != 1 {
		t.Fatal("a later report changed a snapshot already dispatched")
	}
	second.Phase = GestureEnded
	second.Touches[0].Changed, second.Touches[1].Changed = true, false
	tracker.Capture(&second, hit, parent, position)
	move := Event{Phase: GestureChanged, Touches: []Touch{{ID: 9, PagePosition: Point{X: 160, Y: 60}, Changed: true}}}
	tracker.Capture(&move, hit, parent, position)
	if move.TouchGeometry.Positions[10][0] != (Point{X: 60, Y: 60}) {
		t.Fatal("the first finger's responder lost the remaining finger's geometry")
	}
	move.Phase = GestureCancelled
	tracker.Capture(&move, hit, parent, position)
	move.Phase = GestureBegan
	tracker.Capture(&move, hit, parent, position)
	if _, retained := move.TouchGeometry.Positions[10]; retained {
		t.Fatal("a new sequence retained the old responder's path")
	}
}
