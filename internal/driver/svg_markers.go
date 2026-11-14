package driver

import (
	"math"
	"strconv"
	"strings"
)

// svgVertex is a vertex of a marked path, with the directions of the segments that enter and leave it, which auto orientation reads. A direction is zero where no segment enters or leaves.
type svgVertex struct {
	x, y       float32
	inX, inY   float32
	outX, outY float32
}

// markable reports whether markers draw on elements named name: as in SVG 2, only paths, lines, polylines and polygons take them.
func markable(name string) bool {
	switch name {
	case "path", "line", "polyline", "polygon":
		return true
	}
	return false
}

// markers draws the markers el names on the shape in p.local: marker-start on the path's first vertex, marker-end on its last and marker-mid on every other one, inner subpaths' ends included.
func (p *svgParser) markers(el *svgNode, style svgStyle) error {
	if !markable(el.name) {
		return nil
	}
	a := el.attrs
	start := p.ids[svgMarkerRef(a["marker-start"])]
	mid := p.ids[svgMarkerRef(a["marker-mid"])]
	end := p.ids[svgMarkerRef(a["marker-end"])]
	if start == nil && mid == nil && end == nil {
		return nil
	}
	vertices := svgVertices(p.local, p.joins)
	for i, v := range vertices {
		if i == 0 && start != nil {
			if err := p.paintMarker(start, v, true, style); err != nil {
				return err
			}
		}
		if i > 0 && i < len(vertices)-1 && mid != nil {
			if err := p.paintMarker(mid, v, false, style); err != nil {
				return err
			}
		}
		if i == len(vertices)-1 && end != nil {
			if err := p.paintMarker(end, v, false, style); err != nil {
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

// svgVertices lists the vertices of a path's operations: every segment's ends, closing segments' included. joins are the indices of the curves that continue an arc, whose starts are not vertices.
func svgVertices(ops []DrawOp, joins []int) []svgVertex {
	var out []svgVertex
	var x, y float32
	sub := -1 // the current subpath's first vertex
	for i, op := range ops {
		joined := len(joins) > 0 && joins[0] == i
		if joined {
			joins = joins[1:]
		}
		var outX, outY, inX, inY float32
		switch op.Kind {
		case DrawMoveTo:
			out = append(out, svgVertex{x: op.X, y: op.Y})
			x, y, sub = op.X, op.Y, len(out)-1
			continue
		case DrawLineTo:
			outX, outY = op.X-x, op.Y-y
			inX, inY = outX, outY
		case DrawQuadTo:
			outX, outY = svgDirection(op.X1-x, op.Y1-y, op.X-x, op.Y-y)
			inX, inY = svgDirection(op.X-op.X1, op.Y-op.Y1, op.X-x, op.Y-y)
		case DrawCubicTo:
			outX, outY = svgDirection(op.X1-x, op.Y1-y, op.X2-x, op.Y2-y)
			outX, outY = svgDirection(outX, outY, op.X-x, op.Y-y)
			inX, inY = svgDirection(op.X-op.X2, op.Y-op.Y2, op.X-op.X1, op.Y-op.Y1)
			inX, inY = svgDirection(inX, inY, op.X-x, op.Y-y)
		case DrawClosePath:
			if sub < 0 {
				continue
			}
			// The closing segment ends where the subpath started, which it enters, and leaves along the subpath's first segment.
			first := out[sub]
			dx, dy := first.x-x, first.y-y
			out[len(out)-1].outX, out[len(out)-1].outY = dx, dy
			out[sub].inX, out[sub].inY = dx, dy
			out = append(out, svgVertex{x: first.x, y: first.y, inX: dx, inY: dy, outX: first.outX, outY: first.outY})
			x, y, sub = first.x, first.y, len(out)-1
			continue
		default:
			continue
		}
		if sub < 0 {
			out = append(out, svgVertex{x: x, y: y})
			sub = len(out) - 1
		}
		last := &out[len(out)-1]
		if joined {
			last.x, last.y, last.inX, last.inY = op.X, op.Y, inX, inY
		} else {
			last.outX, last.outY = outX, outY
			out = append(out, svgVertex{x: op.X, y: op.Y, inX: inX, inY: inY})
		}
		x, y = op.X, op.Y
	}
	return out
}

// svgDirection returns (x, y), or (nx, ny) when (x, y) is zero: a curve's direction at an end is toward its first control point that is elsewhere.
func svgDirection(x, y, nx, ny float32) (float32, float32) {
	if x == 0 && y == 0 {
		return nx, ny
	}
	return x, y
}

// angle is the direction auto orientation turns a marker at v to, in radians: halfway between the directions entering and leaving it, or the one there is.
func (v svgVertex) angle() float64 {
	inX, inY, outX, outY := v.inX, v.inY, v.outX, v.outY
	if inX == 0 && inY == 0 {
		inX, inY = outX, outY
	}
	if outX == 0 && outY == 0 {
		outX, outY = inX, inY
	}
	in := math.Atan2(float64(inY), float64(inX))
	out := math.Atan2(float64(outY), float64(outX))
	if math.Abs(in-out) > math.Pi {
		in += 2 * math.Pi
	}
	return (in + out) / 2
}

// paintMarker draws marker at the vertex v of a shape drawn with parent; start is set for the marker-start marker, which auto-start-reverse turns around.
func (p *svgParser) paintMarker(marker *svgNode, v svgVertex, start bool, parent svgStyle) error {
	if len(marker.children) == 0 || p.using[marker] || len(p.using) >= maxSVGUses {
		return nil
	}
	if p.visits++; p.visits > maxSVGVisits || len(p.ops) > maxSVGOps {
		return errSVGTooComplex
	}
	a := marker.attrs
	width, height := float32(3), float32(3)
	if n, err := svgLength(a["markerWidth"]); err == nil && strings.TrimSpace(a["markerWidth"]) != "" {
		width = n
	}
	if n, err := svgLength(a["markerHeight"]); err == nil && strings.TrimSpace(a["markerHeight"]) != "" {
		height = n
	}
	unit := float32(1)
	if strings.TrimSpace(a["markerUnits"]) != "userSpaceOnUse" {
		unit = parent.width
	}
	if width <= 0 || height <= 0 || unit <= 0 {
		return nil
	}
	refX, refY := float32(0), float32(0)
	if n, err := svgLength(a["refX"]); err == nil {
		refX = n
	}
	if n, err := svgLength(a["refY"]); err == nil {
		refY = n
	}
	var angle float64
	switch orient := strings.TrimSpace(a["orient"]); orient {
	case "auto":
		angle = v.angle()
	case "auto-start-reverse":
		angle = v.angle()
		if start {
			angle += math.Pi
		}
	default:
		angle = svgAngle(orient)
	}
	sin, cos := math.Sincos(angle)
	m := parent.ctm.mul(svgTranslate(v.x, v.y)).mul(svgMatrix{a: cos, b: sin, c: -sin, d: cos}).mul(svgMatrix{a: float64(unit), d: float64(unit)})
	// The viewBox maps into the marker's width and height, and the content moves so that the reference point, in the viewBox's units, lands on the vertex: a point P draws at scale × (P − ref), and the viewport, where the viewBox's alignment puts it, is the clip.
	sx, sy := float32(1), float32(1)
	clipX, clipY := -refX, -refY
	if nums, err := svgNumbers(a["viewBox"], 4); err == nil {
		if nums[2] <= 0 || nums[3] <= 0 {
			return nil
		}
		sx, sy = width/nums[2], height/nums[3]
		var tx, ty float32
		if ax, ay, uniform, slice := svgAspect(a["preserveAspectRatio"]); uniform {
			s := min(sx, sy)
			if slice {
				s = max(sx, sy)
			}
			sx, sy = s, s
			tx, ty = (width-nums[2]*s)*ax, (height-nums[3]*s)*ay
		}
		clipX, clipY = -(tx + sx*(refX-nums[0])), -(ty + sy*(refY-nums[1]))
	}
	if overflow := strings.TrimSpace(a["overflow"]); overflow != "visible" && overflow != "auto" {
		p.op(DrawOp{Kind: DrawSave})
		p.op(DrawOp{Kind: DrawBeginPath})
		p.emitOps([]DrawOp{{Kind: DrawRect, X: clipX, Y: clipY, W: width, H: height}}, m)
		p.op(DrawOp{Kind: DrawClip})
		defer p.op(DrawOp{Kind: DrawRestore})
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
	style.ctm = m.mul(svgMatrix{a: float64(sx), d: float64(sy)}).mul(svgTranslate(-refX, -refY))
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

// svgAngle reads an angle in degrees, or in grad, rad or turn units, as radians; one it cannot read is 0.
func svgAngle(v string) float64 {
	scale := math.Pi / 180
	for _, u := range []struct {
		unit  string
		scale float64
	}{{"deg", math.Pi / 180}, {"grad", math.Pi / 200}, {"rad", 1}, {"turn", 2 * math.Pi}} {
		if strings.HasSuffix(v, u.unit) {
			v, scale = strings.TrimSpace(v[:len(v)-len(u.unit)]), u.scale
			break
		}
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsInf(n, 0) || n != n {
		return 0
	}
	return n * scale
}

// svgAspect reads preserveAspectRatio: where a viewBox aligns in its viewport on each axis, from 0 for its start to 1 for its end, whether it scales uniformly, which none turns off, and whether it then covers the viewport rather than fits in it.
func svgAspect(v string) (ax, ay float32, uniform, slice bool) {
	f := strings.Fields(v)
	if len(f) > 0 && f[0] == "defer" {
		f = f[1:]
	}
	ax, ay, uniform = 0.5, 0.5, true
	if len(f) == 0 {
		return
	}
	if f[0] == "none" {
		return 0, 0, false, false
	}
	align := func(s string) float32 {
		switch s {
		case "Min":
			return 0
		case "Max":
			return 1
		}
		return 0.5
	}
	if len(f[0]) == 8 && f[0][0] == 'x' && f[0][4] == 'Y' {
		ax, ay = align(f[0][1:4]), align(f[0][5:8])
	}
	return ax, ay, true, len(f) > 1 && f[1] == "slice"
}
