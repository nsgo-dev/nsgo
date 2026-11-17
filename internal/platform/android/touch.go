package android

import (
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// readTouches reads the fingers of an EventTouch the runtime sends: each "id x y changed", in points, joined by ";".
func readTouches(s string) []driver.Touch {
	s, _, _ = strings.Cut(s, "\n")
	var out []driver.Touch
	for _, f := range strings.Split(s, ";") {
		p := strings.Fields(f)
		if len(p) != 4 {
			continue
		}
		id, _ := strconv.Atoi(p[0])
		x, _ := strconv.ParseFloat(p[1], 32)
		y, _ := strconv.ParseFloat(p[2], 32)
		out = append(out, driver.Touch{ID: id, PagePosition: driver.Point{X: driver.Points(x), Y: driver.Points(y)}, Changed: p[3] == "1"})
	}
	return out
}

// readTouchEvent adds the runtime's presentation snapshot to the finger list: t <finger> <target>, then p <view> <x0> <y0> ... in finger order, in points.
func readTouchEvent(s string) ([]driver.Touch, *driver.TouchGeometry) {
	points, lines, snapshot := strings.Cut(s, "\n")
	touches := readTouches(points)
	if !snapshot {
		return touches, nil
	}
	g := &driver.TouchGeometry{Targets: map[int]driver.NodeID{}, Positions: map[driver.NodeID][]driver.Point{}}
	for _, line := range strings.Split(lines, "\n") {
		p := strings.Fields(line)
		if len(p) < 2 {
			continue
		}
		id, err := strconv.Atoi(p[1])
		if err != nil {
			continue
		}
		switch p[0] {
		case "t":
			if len(p) == 3 {
				target, err := strconv.ParseUint(p[2], 10, 32)
				if err == nil {
					g.Targets[id] = driver.NodeID(target)
				}
			}
		case "p":
			if len(p) != 2+2*len(touches) {
				continue
			}
			var positions []driver.Point
			for i := 2; i < len(p); i += 2 {
				x, ex := strconv.ParseFloat(p[i], 32)
				y, ey := strconv.ParseFloat(p[i+1], 32)
				if ex != nil || ey != nil {
					break
				}
				positions = append(positions, driver.Point{X: driver.Points(x), Y: driver.Points(y)})
			}
			if len(positions) == len(touches) {
				g.Positions[driver.NodeID(id)] = positions
			}
		}
	}
	return touches, g
}
