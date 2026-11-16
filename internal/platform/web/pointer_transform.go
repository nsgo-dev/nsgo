//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// layoutOrigin is the untransformed border-box origin in document
// coordinates. offsetParent can skip static containers, so subtracting
// two such origins also works for an embedding page's wrappers.
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

// elementMatrix reads the browser's current values, including CSS
// transitions and Web Animations, rather than a style's destination.
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

// clientMatrix composes element and ancestor transforms with actual DOM
// placement and scrolling. The browser remains responsible for picking;
// this supplies coordinates in the picked view's untransformed viewport.
func clientMatrix(el js.Value) driver.Matrix3D {
	m := elementMatrix(el)
	for parent := el.Get("parentElement"); parent.Truthy(); parent = el.Get("parentElement") {
		x, y := layoutOrigin(el)
		px, py := layoutOrigin(parent)
		placed := driver.Translate3D(x-px-parent.Get("scrollLeft").Float(), y-py-parent.Get("scrollTop").Float(), 0).Mul(m)
		cs := js.Global().Call("getComputedStyle", parent)
		if depth := cssNumber(cs.Get("perspective").String(), 0); depth > 0 {
			origin := strings.Fields(cs.Get("perspectiveOrigin").String())
			if len(origin) >= 2 {
				ox, oy := cssNumber(origin[0], 0), cssNumber(origin[1], 0)
				placed = driver.Translate3D(ox, oy, 0).Mul(driver.Perspective3D(depth)).Mul(driver.Translate3D(-ox, -oy, 0)).Mul(placed)
			}
		}
		m = elementMatrix(parent).Mul(placed)
		el = parent
	}
	x, y := layoutOrigin(el)
	return driver.Translate3D(x, y, 0).Mul(m)
}

func clientPoint(el js.Value, x, y float64) (float64, float64) {
	p, ok := clientMatrix(el).Unproject(driver.Point{X: driver.Points(x), Y: driver.Points(y)})
	if !ok {
		return math.Inf(1), math.Inf(1)
	}
	return float64(p.X), float64(p.Y)
}
