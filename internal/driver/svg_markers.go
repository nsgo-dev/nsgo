package driver

import (
	"math"
	"strconv"
	"strings"
)

// svgMarkerPoint is a path vertex and the directions of the segments that
// enter and leave it. SVG markers use these directions for auto orientation.
type svgMarkerPoint struct {
	x, y       float32
	inX, inY   float32
	outX, outY float32
}

// markers paints marker-start, marker-mid and marker-end on a shape. The
// geometry is taken from the already parsed path, so markers work for all
// path commands, including curves and the shapes converted to paths.
func (p *svgParser) markers(el *svgNode, style svgStyle) error {
	a := el.attrs
	start := p.ids[svgMarkerRef(a["marker-start"])]
	mid := p.ids[svgMarkerRef(a["marker-mid"])]
	end := p.ids[svgMarkerRef(a["marker-end"])]
	if start == nil && mid == nil && end == nil {
		return nil
	}
	paths := svgMarkerPaths(p.local)
	for _, path := range paths {
		if len(path) == 0 {
			continue
		}
		if start != nil {
			if err := p.paintMarker(start, path[0], path[0].outX, path[0].outY, style, false); err != nil {
				return err
			}
		}
		if mid != nil {
			for i := 1; i+1 < len(path); i++ {
				v := path[i]
				inX, inY, outX, outY := v.inX, v.inY, v.outX, v.outY
				if outX == 0 && outY == 0 {
					outX, outY = inX, inY
				}
				if inX == 0 && inY == 0 {
					inX, inY = outX, outY
				}
				if err := p.paintMarker(mid, v, inX+outX, inY+outY, style, false); err != nil {
					return err
				}
			}
		}
		if end != nil {
			v := path[len(path)-1]
			if err := p.paintMarker(end, v, v.inX, v.inY, style, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func svgMarkerRef(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "url(") {
		return ""
	}
	if i := strings.IndexByte(v, ')'); i >= 0 {
		return svgHref(map[string]string{"href": strings.TrimSpace(v[4:i])})
	}
	return ""
}

// svgMarkerPaths splits a shape's path operations into subpaths and records
// the tangent at each vertex. Non-path geometry is already represented by
// the same operations by svgParser.emitPath.
func svgMarkerPaths(ops []DrawOp) [][]svgMarkerPoint {
	var out [][]svgMarkerPoint
	var path []svgMarkerPoint
	var startX, startY, x, y float32
	finish := func() {
		if len(path) > 0 {
			out = append(out, path)
			path = nil
		}
	}
	for _, op := range ops {
		switch op.Kind {
		case DrawBeginPath:
			finish()
		case DrawMoveTo:
			finish()
			x, y, startX, startY = op.X, op.Y, op.X, op.Y
			path = append(path, svgMarkerPoint{x: x, y: y})
		case DrawLineTo:
			dx, dy := op.X-x, op.Y-y
			if len(path) > 0 {
				path[len(path)-1].outX, path[len(path)-1].outY = dx, dy
			}
			path = append(path, svgMarkerPoint{x: op.X, y: op.Y, inX: dx, inY: dy})
			x, y = op.X, op.Y
		case DrawQuadTo:
			outX, outY := op.X1-x, op.Y1-y
			inX, inY := op.X-op.X1, op.Y-op.Y1
			if len(path) > 0 {
				path[len(path)-1].outX, path[len(path)-1].outY = outX, outY
			}
			path = append(path, svgMarkerPoint{x: op.X, y: op.Y, inX: inX, inY: inY})
			x, y = op.X, op.Y
		case DrawCubicTo:
			outX, outY := op.X1-x, op.Y1-y
			inX, inY := op.X-op.X2, op.Y-op.Y2
			if len(path) > 0 {
				path[len(path)-1].outX, path[len(path)-1].outY = outX, outY
			}
			path = append(path, svgMarkerPoint{x: op.X, y: op.Y, inX: inX, inY: inY})
			x, y = op.X, op.Y
		case DrawClosePath:
			dx, dy := startX-x, startY-y
			if len(path) > 0 {
				path[len(path)-1].outX, path[len(path)-1].outY = dx, dy
				path[0].inX, path[0].inY = dx, dy
			}
			x, y = startX, startY
		}
	}
	finish()
	return out
}

func (p *svgParser) paintMarker(marker *svgNode, point svgMarkerPoint, dx, dy float32, parent svgStyle, end bool) error {
	if marker == nil || len(marker.children) == 0 || (dx == 0 && dy == 0) || p.using[marker] || len(p.using) >= maxSVGUses {
		return nil
	}
	if p.visits++; p.visits > maxSVGVisits || len(p.ops) > maxSVGOps {
		return errSVGTooComplex
	}
	width, height := float32(3), float32(3)
	if n, err := svgLength(marker.attrs["markerWidth"]); err == nil && n > 0 {
		width = n
	}
	if n, err := svgLength(marker.attrs["markerHeight"]); err == nil && n > 0 {
		height = n
	}
	refX, refY := float32(0), float32(0)
	if n, err := svgLength(marker.attrs["refX"]); err == nil {
		refX = n
	}
	if n, err := svgLength(marker.attrs["refY"]); err == nil {
		refY = n
	}
	angle := float64(math.Atan2(float64(dy), float64(dx)))
	if orient := strings.TrimSpace(marker.attrs["orient"]); orient != "" && orient != "auto" && orient != "auto-start-reverse" {
		if n, err := strconv.ParseFloat(orient, 64); err == nil {
			angle = n * math.Pi / 180
		}
	} else if orient == "auto-start-reverse" && !end {
		angle += math.Pi
	}
	unit := float32(1)
	if strings.TrimSpace(marker.attrs["markerUnits"]) != "userSpaceOnUse" {
		unit = parent.width
		if unit <= 0 {
			unit = 1
		}
	}
	m := parent.ctm.mul(svgTranslate(point.x, point.y)).mul(svgMatrix{a: math.Cos(angle), b: math.Sin(angle), c: -math.Sin(angle), d: math.Cos(angle)})
	viewBox := marker.attrs["viewBox"]
	if nums, err := svgNumbers(viewBox, 4); err == nil && nums[2] > 0 && nums[3] > 0 {
		m = m.mul(svgMatrix{a: float64(unit * width / nums[2]), d: float64(unit * height / nums[3])})
		m = m.mul(svgTranslate(-nums[0]-refX, -nums[1]-refY))
	} else {
		m = m.mul(svgMatrix{a: float64(unit), d: float64(unit)}).mul(svgTranslate(-refX, -refY))
	}
	style := defaultSVGStyle()
	if err := style.apply(marker.attrs); err != nil {
		return err
	}
	for _, child := range marker.children {
		if child.attrs["fill"] == "context-stroke" {
			style.fill = parent.stroke
		}
		if child.attrs["stroke"] == "context-fill" {
			style.stroke = parent.fill
		}
	}
	style.ctm = m
	if p.using == nil {
		p.using = map[*svgNode]bool{}
	}
	p.using[marker] = true
	defer delete(p.using, marker)
	for _, child := range marker.children {
		if err := p.element(child, style, false); err != nil {
			return err
		}
	}
	return nil
}
