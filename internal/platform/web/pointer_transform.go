//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// layoutOrigin is the untransformed border-box origin in document coordinates. offsetParent can skip static containers, so subtracting two such origins also works for an embedding page's wrappers.
func layoutOrigin(el js.Value) (x, y float64) {
	for el.Truthy() {
		x += el.Get("offsetLeft").Float()
		y += el.Get("offsetTop").Float()
		el = el.Get("offsetParent")
		if el.Truthy() {
			x += el.Get("clientLeft").Float()
			y += el.Get("clientTop").Float()
		}
	}
	return
}

func cssNumber(s string, size float64) float64 {
	if strings.HasSuffix(s, "%") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		return v * size / 100
	}
	v, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(s, "px"), "deg"), 64)
	return v
}

// elementMatrix reads the browser's current values, including CSS transitions and Web Animations, rather than a style's destination.
func elementMatrix(el js.Value) driver.Matrix3D {
	cs := js.Global().Call("getComputedStyle", el)
	m := driver.Identity3D
	if transform := cs.Get("transform").String(); transform != "none" && transform != "" {
		dom := js.Global().Get("DOMMatrixReadOnly").New(transform)
		for c := range 4 {
			for r := range 4 {
				m[c*4+r] = dom.Get("m" + strconv.Itoa(c+1) + strconv.Itoa(r+1)).Float()
			}
		}
	}
	// Keyframes use the individual translate, rotate and scale properties.
	tr := driver.Transform{}
	if fields := strings.Fields(cs.Get("translate").String()); len(fields) > 0 && fields[0] != "none" {
		tr.TranslateX = driver.Points(cssNumber(fields[0], el.Get("offsetWidth").Float()))
		if len(fields) > 1 {
			tr.TranslateY = driver.Points(cssNumber(fields[1], el.Get("offsetHeight").Float()))
		}
	}
	if fields := strings.Fields(cs.Get("rotate").String()); len(fields) > 0 && fields[0] != "none" {
		tr.Rotate = float32(cssNumber(fields[len(fields)-1], 0))
	}
	if fields := strings.Fields(cs.Get("scale").String()); len(fields) > 0 && fields[0] != "none" {
		tr.ScaleX = float32(cssNumber(fields[0], 0))
		tr.ScaleY = tr.ScaleX
		if len(fields) > 1 {
			tr.ScaleY = float32(cssNumber(fields[1], 0))
		}
	}
	m = tr.Matrix(driver.Point{}).Mul(m)
	var origin [3]float64
	for i, f := range strings.Fields(cs.Get("transformOrigin").String()) {
		if i < len(origin) {
			origin[i] = cssNumber(f, 0)
		}
	}
	return driver.Translate3D(origin[0], origin[1], origin[2]).Mul(m).
		Mul(driver.Translate3D(-origin[0], -origin[1], -origin[2]))
}

// clientMatrix maps el's untransformed border box to client coordinates, through its transforms and its ancestors', their DOM placement and scrolling. The browser remains responsible for picking; this supplies coordinates in the picked view's untransformed viewport. Above the topmost transformed element, the placement comes from getBoundingClientRect, which keeps fractions; offsetLeft and offsetTop, whole pixels, place only what a transform holds.
func clientMatrix(el js.Value) driver.Matrix3D {
	var chain []js.Value
	var mats []driver.Matrix3D
	top := -1
	for e := el; e.Truthy(); e = e.Get("parentElement") {
		m := elementMatrix(e)
		if m != driver.Identity3D {
			top = len(chain)
		}
		chain, mats = append(chain, e), append(mats, m)
	}
	if top < 0 {
		r := el.Call("getBoundingClientRect")
		return driver.Translate3D(r.Get("left").Float(), r.Get("top").Float(), 0)
	}
	m := mats[0]
	for i := 0; i <= top; i++ {
		if i+1 == len(chain) {
			x, y := layoutOrigin(chain[i])
			return driver.Translate3D(x, y, 0).Mul(m)
		}
		e, parent := chain[i], chain[i+1]
		x, y := layoutOrigin(e)
		px, py := layoutOrigin(parent)
		sx, sy := parent.Get("scrollLeft").Float(), parent.Get("scrollTop").Float()
		if i == top && parent.Equal(js.Global().Get("document").Get("scrollingElement")) {
			// The page's scrolling moves its own box, which the rectangle below has.
			sx, sy = 0, 0
		}
		m = perspectiveOf(parent).Mul(driver.Translate3D(x-px-sx, y-py-sy, 0).Mul(m))
		if i < top {
			m = mats[i+1].Mul(m)
		}
	}
	r := chain[top+1].Call("getBoundingClientRect")
	return driver.Translate3D(r.Get("left").Float(), r.Get("top").Float(), 0).Mul(m)
}

// perspectiveOf is the perspective parent gives its children's transforms, or the identity.
func perspectiveOf(parent js.Value) driver.Matrix3D {
	cs := js.Global().Call("getComputedStyle", parent)
	depth := cssNumber(cs.Get("perspective").String(), 0)
	origin := strings.Fields(cs.Get("perspectiveOrigin").String())
	if depth <= 0 || len(origin) < 2 {
		return driver.Identity3D
	}
	ox, oy := cssNumber(origin[0], 0), cssNumber(origin[1], 0)
	return driver.Translate3D(ox, oy, 0).Mul(driver.Perspective3D(depth)).Mul(driver.Translate3D(-ox, -oy, 0))
}

func clientPoint(el js.Value, x, y float64) (float64, float64) {
	p, ok := clientMatrix(el).Unproject(driver.Point{X: driver.Points(x), Y: driver.Points(y)})
	if !ok {
		return math.Inf(1), math.Inf(1)
	}
	return float64(p.X), float64(p.Y)
}
