package camera

import (
	"math"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

type harness struct {
	app     *nsgotest.App
	state   State
	presses []core.Point
	hovers  []core.Point
	take    bool
}

// newHarness shows a 1200 by 800 board on a 900 by 700 plane.
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{}
	h.app = nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "plane", State: &h.state, Style: core.Style{Width: core.Pt(900), Height: core.Pt(700)},
			Content: core.View{TestID: "board", Style: core.Style{Width: core.Pt(1200), Height: core.Pt(800)}},
			OnPress: func(at core.Point, e core.PressEvent) bool { h.presses = append(h.presses, at); return h.take },
			OnHover: func(at core.Point, over bool) {
				if over {
					h.hovers = append(h.hovers, at)
				}
			},
			Children: []core.Node{core.View{TestID: "tools", Style: core.Style{Position: core.Absolute, Top: core.Pt(8), Right: core.Pt(8),
				Width: core.Pt(40), Height: core.Pt(20)}}}}
	}, nsgotest.Size(900, 700), nsgotest.Environment(driver.Environment{ReduceMotion: true, Idiom: driver.IdiomDesktop}))
	h.app.Settle()
	return h
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func TestViewFitsTheContent(t *testing.T) {
	h := newHarness(t)
	// The board shows whole and centered, inside the margin.
	x, y, zoom := h.state.Camera()
	if want := (900 - 64) / 1200.0; !near(zoom, want) || !near(x, 32) || !near(y, (700-800*want)/2) {
		t.Fatalf("the camera is at %v,%v ×%v", x, y, zoom)
	}
	if h.state.Moved() {
		t.Fatal("a fit counts as a move")
	}
	// A part of it shows as the user moving there would.
	h.app.Post(func() {
		h.state.FitBounds(core.Bounds{Origin: core.Point{X: 600, Y: 400}, Size: core.Size{Width: 200, Height: 100}})
	})
	h.app.Settle()
	if p := h.state.ToView(core.Point{X: 700, Y: 450}); !near(float64(p.X), 450) || !near(float64(p.Y), 350) || !near(h.state.Zoom(), 1) || !h.state.Moved() {
		t.Fatalf("the part's middle shows at %v ×%v", p, h.state.Zoom())
	}
	h.app.Post(h.state.Fit)
	h.app.Settle()
	if _, _, z := h.state.Camera(); !near(z, zoom) || h.state.Moved() {
		t.Fatalf("fit zoom %v, want %v", z, zoom)
	}
	// What sits over the plane stays put.
	if f := h.app.Find("id:tools").Frame; f.Origin.X != 900-48 || f.Origin.Y != 8 {
		t.Fatalf("the tools are at %v", f.Origin)
	}
}

func TestViewHandsOnThePointer(t *testing.T) {
	h := newHarness(t)
	_, _, zoom := h.state.Camera()
	at := h.state.ToView(core.Point{X: 100, Y: 200})
	h.app.PressAt("id:plane", at)
	if len(h.presses) != 1 || !near(float64(h.presses[0].X), 100) || !near(float64(h.presses[0].Y), 200) {
		t.Fatalf("pressed at %v", h.presses)
	}
	h.app.HoverAt("id:plane-pointer", at)
	if n := len(h.hovers); n == 0 || !near(float64(h.hovers[n-1].X), 100) {
		t.Fatalf("hovered at %v", h.hovers)
	}
	// A double press the app leaves zooms in there; one it takes does not.
	h.app.Advance(time.Second)
	h.app.PressAt("id:plane", at)
	h.app.PressAt("id:plane", at)
	if z := h.state.Zoom(); !near(z/zoom, 2) {
		t.Fatalf("a double press zoomed to %v from %v", z, zoom)
	}
	h.take = true
	h.app.Advance(time.Second)
	zoom = h.state.Zoom()
	h.app.PressAt("id:plane", at)
	h.app.PressAt("id:plane", at)
	if !near(h.state.Zoom(), zoom) {
		t.Fatal("a double press the app took zoomed")
	}
}

func TestViewContentTakesThePointerThroughTheCamera(t *testing.T) {
	var state State
	var presses []core.PressEvent
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "plane", State: &state, Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)},
			Content: core.View{Style: core.Style{Width: core.Pt(1000), Height: core.Pt(800)}, Children: []core.Node{
				core.View{TestID: "button", Style: core.Style{Position: core.Absolute, Left: core.Pt(300), Top: core.Pt(200),
					Width: core.Pt(40), Height: core.Pt(40)}, OnPress: func(e core.PressEvent) { presses = append(presses, e) }},
			}}}
	}, nsgotest.Size(400, 300), nsgotest.Environment(driver.Environment{ReduceMotion: true}))
	press := func() {
		t.Helper()
		app.PressAt("id:plane", state.ToView(core.Point{X: 320, Y: 220}))
		if len(presses) == 0 || !near(float64(presses[len(presses)-1].Position.X), 20) || !near(float64(presses[len(presses)-1].Position.Y), 20) {
			t.Fatalf("content press = %+v, want local (20,20)", presses)
		}
	}
	press()
	app.Post(state.ZoomIn)
	app.Settle()
	press()
	if len(presses) != 2 {
		t.Fatalf("content presses = %d, want 2", len(presses))
	}
}

func TestViewPansAndZooms(t *testing.T) {
	h := newHarness(t)
	x, y, zoom := h.state.Camera()
	h.app.Pan("id:plane", driver.Point{X: 20, Y: -20}, driver.Point{X: 60, Y: -50})
	if nx, ny, _ := h.state.Camera(); !near(nx-x, 60) || !near(ny-y, -50) || !h.state.Moved() {
		t.Fatalf("a pan moved the camera from %v,%v to %v,%v", x, y, nx, ny)
	}
	if len(h.presses) != 0 {
		t.Fatal("panning pressed")
	}
	h.app.Post(h.state.ZoomIn)
	h.app.Settle()
	if !near(h.state.Zoom()/zoom, zoomStep) {
		t.Fatalf("zoom in went to %v from %v", h.state.Zoom(), zoom)
	}
	h.app.Post(h.state.ZoomOut)
	h.app.Settle()
	if !near(h.state.Zoom(), zoom) {
		t.Fatalf("zoom out went to %v", h.state.Zoom())
	}
	// Center keeps the zoom.
	h.app.Post(func() {
		h.state.Center(core.Bounds{Origin: core.Point{X: 1100, Y: 700}, Size: core.Size{Width: 100, Height: 100}})
	})
	h.app.Settle()
	if p := h.state.ToView(core.Point{X: 1150, Y: 750}); !near(float64(p.X), 450) || !near(float64(p.Y), 350) || !near(h.state.Zoom(), zoom) {
		t.Fatalf("the centered part shows at %v", p)
	}
}

func TestViewWheelAndTouchpad(t *testing.T) {
	h := newHarness(t)
	zoom := h.state.Zoom()
	// A wheel's notch zooms in by ×1.5 over 120 points.
	h.app.Wheel("id:plane", driver.Point{Y: 120}, driver.Modifiers{})
	if got := h.state.Zoom(); !near(got/zoom, 1.5) {
		t.Fatalf("a notch zoomed %v to %v", zoom, got)
	}
	// Two fingers pan; a sideways wheel pans across.
	x, y, z := h.state.Camera()
	h.app.TouchpadScroll("id:plane", driver.Point{}, driver.Point{X: 10, Y: 5}, driver.Point{X: 20, Y: -15})
	if nx, ny, nz := h.state.Camera(); !near(nx-x, 30) || !near(ny-y, -10) || nz != z {
		t.Fatalf("the touchpad moved the camera from %v,%v to %v,%v", x, y, nx, ny)
	}
	x, _, _ = h.state.Camera()
	h.app.Wheel("id:plane", driver.Point{X: -40}, driver.Modifiers{})
	if nx, _, nz := h.state.Camera(); !near(nx-x, -40) || nz != z {
		t.Fatalf("a sideways wheel moved the camera from %v to %v", x, nx)
	}
	// A notch with Control held zooms too, and a pinch around its center.
	zoom = h.state.Zoom()
	h.app.Wheel("id:plane", driver.Point{Y: -120}, driver.Modifiers{Control: true})
	if got := h.state.Zoom(); !near(got/zoom, 1/1.5) {
		t.Fatalf("a Control-scroll zoomed %v to %v", zoom, got)
	}
	zoom = h.state.Zoom()
	h.app.Pinch("id:plane", 1, 2)
	if got := h.state.Zoom(); !near(got/zoom, 2) {
		t.Fatalf("a pinch zoomed %v to %v", zoom, got)
	}
}

func TestViewShownAgainKeepsItsCamera(t *testing.T) {
	var state State
	var shown *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shown = core.NewSignal(cx, true)
		return core.Show{When: shown.Get, Then: func(*core.Scope) core.Node {
			return View{State: &state, Style: core.Style{Width: core.Pt(900), Height: core.Pt(700)},
				Content: core.View{Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)}}}
		}}
	}, nsgotest.Size(900, 700), nsgotest.Environment(driver.Environment{ReduceMotion: true, Idiom: driver.IdiomDesktop}))
	app.Settle()
	app.Post(state.ZoomIn)
	app.Settle()
	x, y, zoom := state.Camera()
	app.Post(func() { shown.Set(false) })
	app.Settle()
	app.Post(func() { shown.Set(true) })
	app.Settle()
	if nx, ny, nz := state.Camera(); nx != x || ny != y || nz != zoom {
		t.Fatalf("shown again the camera moved from %v,%v ×%v to %v,%v ×%v", x, y, zoom, nx, ny, nz)
	}
}
