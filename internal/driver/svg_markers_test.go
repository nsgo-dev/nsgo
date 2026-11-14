package driver

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

// svgLines returns the lines ops draw, as from and to points.
func svgLines(ops []DrawOp) [][4]float32 {
	var out [][4]float32
	var x, y float32
	for _, op := range ops {
		switch op.Kind {
		case DrawMoveTo:
			x, y = op.X, op.Y
		case DrawLineTo:
			out = append(out, [4]float32{x, y, op.X, op.Y})
			x, y = op.X, op.Y
		}
	}
	return out
}

func nearLines(got, want [][4]float32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		for j := range got[i] {
			if math.Abs(float64(got[i][j]-want[i][j])) > 1e-3 {
				return false
			}
		}
	}
	return true
}

// lineMarkerDoc marks path, which paints nothing itself, with a marker that draws a 10-unit line along its x axis.
func lineMarkerDoc(orient, path string) string {
	return fmt.Sprintf(`<svg viewBox="0 0 200 200" fill="none">
		<marker id="m" markerUnits="userSpaceOnUse" overflow="visible" orient="%s"><path d="M0 0L10 0" stroke="red"/></marker>
		%s
	</svg>`, orient, path)
}

func TestSVGMarkerIssueReproduction(t *testing.T) {
	// Marker content is placed relative to the marker's reference point, (0, 0) here, at each vertex.
	ops := svgOps(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 20">
		<marker id="dot" markerWidth="10" markerHeight="10"><circle cx="5" cy="5" r="4" fill="red"/></marker>
		<path fill="none" marker-mid="url(#dot)" d="M0 5h30 30 30"/>
	</svg>`)
	var dots [][2]float32
	for i, op := range ops {
		if op.Kind != DrawEllipse {
			continue
		}
		dots = append(dots, [2]float32{op.X, op.Y})
		if op.R != 4 || op.RY != 4 || ops[i+1].Kind != DrawFillColor || ops[i+1].Color != RGB(0xff0000) {
			t.Fatalf("dot = %+v then %+v, want a red circle of radius 4", op, ops[i+1])
		}
	}
	if len(dots) != 2 || dots[0] != [2]float32{35, 10} || dots[1] != [2]float32{65, 10} {
		t.Fatalf("dots at %v, want (35, 10) and (65, 10)", dots)
	}
}

func TestSVGMarkerPlacement(t *testing.T) {
	for _, tt := range []struct {
		name, orient, path string
		want               [][4]float32
	}{
		{"no orient keeps the marker upright", "", `<path d="M0 0L0 50" marker-end="url(#m)"/>`, [][4]float32{{0, 50, 10, 50}}},
		{"auto follows the path", "auto", `<path d="M0 0L0 50" marker-end="url(#m)"/>`, [][4]float32{{0, 50, 0, 60}}},
		{"an angle turns the marker", "90deg", `<path d="M0 0L50 0" marker-end="url(#m)"/>`, [][4]float32{{50, 0, 50, 10}}},
		{"auto-start-reverse turns the start marker", "auto-start-reverse", `<path d="M0 50L50 50L100 50" marker-start="url(#m)"/>`, [][4]float32{{0, 50, -10, 50}}},
		{"auto-start-reverse leaves mid markers", "auto-start-reverse", `<path d="M0 50L50 50L100 50" marker-mid="url(#m)"/>`, [][4]float32{{50, 50, 60, 50}}},
		{"auto-start-reverse leaves end markers", "auto-start-reverse", `<path d="M0 50L50 50L100 50" marker-end="url(#m)"/>`, [][4]float32{{100, 50, 110, 50}}},
		{"mid markers bisect the corner", "auto", `<path d="M0 0L100 0L100 1" marker-mid="url(#m)"/>`, [][4]float32{{100, 0, 100 + 5*math.Sqrt2, 5 * math.Sqrt2}}},
		{"auto follows a curve's tangent", "auto", `<path d="M0 0C10 0 20 10 20 20" marker-start="url(#m)" marker-end="url(#m)"/>`, [][4]float32{{0, 0, 10, 0}, {20, 20, 20, 30}}},
		{"start and end mark the whole path once", "", `<path d="M0 0L10 0M0 10L10 10" marker-start="url(#m)" marker-end="url(#m)"/>`, [][4]float32{{0, 0, 10, 0}, {10, 10, 20, 10}}},
		{"inner subpaths' ends are mid vertices", "", `<path d="M0 0L10 0M0 10L10 10" marker-mid="url(#m)"/>`, [][4]float32{{10, 0, 20, 0}, {0, 10, 10, 10}}},
		{"an arc is one segment", "", `<path d="M0 50A25 25 0 0 1 50 50" marker-mid="url(#m)"/>`, nil},
		{"an arc ends along its tangent", "auto", `<path d="M0 50A25 25 0 0 1 50 50" marker-end="url(#m)"/>`, [][4]float32{{50, 50, 50, 60}}},
		{"a polygon ends where it closes", "", `<polygon points="0,0 10,0 10,10" marker-end="url(#m)"/>`, [][4]float32{{0, 0, 10, 0}}},
		{"a polygon's last point is a mid vertex", "", `<polygon points="0,0 10,0 10,10" marker-mid="url(#m)"/>`, [][4]float32{{10, 0, 20, 0}, {10, 10, 20, 10}}},
		{"lines and polylines take markers", "", `<line x1="0" y1="0" x2="5" y2="0" marker-end="url(#m)"/><polyline points="0,20 5,20" marker-start="url(#m)"/>`, [][4]float32{{5, 0, 15, 0}, {0, 20, 10, 20}}},
		{"other shapes do not", "", `<rect width="10" height="10" marker-start="url(#m)"/><circle r="5" marker-start="url(#m)"/>`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := svgLines(svgOps(t, lineMarkerDoc(tt.orient, tt.path))); !nearLines(got, tt.want) {
				t.Fatalf("lines = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSVGMarkerViewBox(t *testing.T) {
	ellipse := func(t *testing.T, doc string) DrawOp {
		t.Helper()
		for _, op := range svgOps(t, doc) {
			if op.Kind == DrawEllipse {
				return op
			}
		}
		t.Fatal("no ellipse")
		return DrawOp{}
	}
	for _, tt := range []struct {
		name, marker, path string
		x, y, rx, ry       float32
	}{
		{"a centered viewBox", `viewBox="-5 -5 10 10" markerWidth="10" markerHeight="10"><circle r="5"/>`, `d="M50 50L60 50"`, 50, 50, 5, 5},
		{"viewBox meets its viewport", `viewBox="0 0 10 20" markerWidth="10" markerHeight="10" refX="5" refY="10"><circle cx="5" cy="10" r="5"/>`, `d="M50 50L60 50"`, 50, 50, 2.5, 2.5},
		{"preserveAspectRatio none", `viewBox="0 0 10 20" markerWidth="10" markerHeight="10" refX="5" refY="10" preserveAspectRatio="none"><circle cx="5" cy="10" r="5"/>`, `d="M50 50L60 50"`, 50, 50, 5, 2.5},
		{"refX scales with the stroke width", `refX="1"><circle r="1"/>`, `d="M50 50L60 50" stroke-width="2"`, 48, 50, 2, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			op := ellipse(t, `<svg viewBox="0 0 100 100"><marker id="m" overflow="visible" `+tt.marker+`</marker><path fill="none" marker-start="url(#m)" `+tt.path+`/></svg>`)
			if op.X != tt.x || op.Y != tt.y || op.R != tt.rx || op.RY != tt.ry {
				t.Fatalf("ellipse = (%v, %v) radii %v, %v, want (%v, %v) radii %v, %v", op.X, op.Y, op.R, op.RY, tt.x, tt.y, tt.rx, tt.ry)
			}
		})
	}
}

func TestSVGMarkerClip(t *testing.T) {
	doc := `<svg viewBox="0 0 100 100"><marker id="m" markerWidth="4" markerHeight="3" refX="1" %s><path d="M0 0L20 0" stroke="red"/></marker><path d="M50 50L60 50" fill="none" marker-start="url(#m)"/></svg>`
	ops := svgOps(t, fmt.Sprintf(doc, ""))
	want := []DrawOp{{Kind: DrawSave}, {Kind: DrawBeginPath}, {Kind: DrawRect, X: 49, Y: 50, W: 4, H: 3}, {Kind: DrawClip}}
	for i, w := range want {
		if op := ops[i]; op.Kind != w.Kind || op.X != w.X || op.Y != w.Y || op.W != w.W || op.H != w.H {
			t.Fatalf("op %d = %+v, want %+v", i, ops[i], w)
		}
	}
	if last := ops[len(ops)-1]; last.Kind != DrawRestore {
		t.Fatalf("last op = %v, want the clip's restore", last.Kind)
	}
	for _, op := range svgOps(t, fmt.Sprintf(doc, `overflow="visible"`)) {
		if op.Kind == DrawClip {
			t.Fatal("a marker with overflow visible clipped its content")
		}
	}
}

func TestSVGMarkerLimits(t *testing.T) {
	for _, doc := range []string{
		`<svg viewBox="0 0 10 10"><marker id="m"><path d="M0 0L1 1" fill="none" marker-end="url(#m)"/></marker><path d="M0 0L5 5" stroke="blue" marker-end="url(#m)"/></svg>`,
		`<svg viewBox="0 0 10 10"><marker id="m"><path d="M0 0L1 1" stroke="red" marker-end="url(#m)"/></marker><path d="M0 0L5 5" stroke="blue" marker-end="url(#m)"/></svg>`,
		`<svg viewBox="0 0 10 10"><marker id="a"><path d="M0 0L1 1" stroke="red" marker-end="url(#b)"/></marker><marker id="b"><path d="M0 0L1 1" stroke="red" marker-end="url(#a)"/></marker><path d="M0 0L5 5" marker-end="url(#a)"/></svg>`,
	} {
		// A marker never draws inside itself.
		if ops := svgOps(t, doc); len(ops) > 100 {
			t.Fatalf("a marker drawn inside itself recorded %d operations", len(ops))
		}
	}
	// Seven markers, each with ten mid vertices marked by the next, ask for ten million elements.
	doc := `<svg viewBox="0 0 10 10">`
	for i := range 7 {
		doc += fmt.Sprintf(`<marker id="m%d" overflow="visible"><path fill="none" d="M0 0h1h1h1h1h1h1h1h1h1h1h1" marker-mid="url(#m%d)"/></marker>`, i, i+1)
	}
	doc += `<path fill="none" d="M0 0h1h1h1h1h1h1h1h1h1h1h1" marker-mid="url(#m0)"/></svg>`
	if _, err := ParseSVG([]byte(doc)); !errors.Is(err, errSVGTooComplex) {
		t.Fatalf("ParseSVG = %v, want %v", err, errSVGTooComplex)
	}
	if _, err := ParseSVG([]byte(`<svg viewBox="0 0 10 10"><marker id="m"><path d="M0 0 X"/></marker><path d="M0 0L5 5" marker-end="url(#m)"/></svg>`)); err == nil {
		t.Fatal("ParseSVG drew a marker with bad path data without an error")
	}
}
