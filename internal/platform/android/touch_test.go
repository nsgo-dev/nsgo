package android

import (
	"reflect"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestTouchEventReadsNativeCoordinatesForEveryFinger(t *testing.T) {
	touches, g := readTouchEvent("7 120 80 1;9 160 100 0\nt 7 12\np 12 10 20 30 40\np 4 60 40 80 50\n")
	if len(touches) != 2 || touches[0].ID != 7 || !touches[0].Changed || touches[1].Changed ||
		touches[1].PagePosition != (driver.Point{X: 160, Y: 100}) {
		t.Fatalf("touches = %+v", touches)
	}
	if g == nil || g.Targets[7] != 12 || !reflect.DeepEqual(g.Positions[12], []driver.Point{{X: 10, Y: 20}, {X: 30, Y: 40}}) ||
		g.Positions[4][1] != (driver.Point{X: 80, Y: 50}) {
		t.Fatalf("geometry = %+v", g)
	}
	_, g = readTouchEvent("7 120 80 1\nt 7 0\np 12 10\np 4 invalid 20\n")
	if target, reported := g.Targets[7]; !reported || target != 0 || len(g.Positions) != 0 {
		t.Fatalf("invalid positions or a native miss = %+v", g)
	}
	_, g = readTouchEvent("7 120 80 1")
	if g != nil {
		t.Fatal("a legacy report acquired native geometry")
	}
}
