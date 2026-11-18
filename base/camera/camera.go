// Package camera is a plane the user pans and zooms, such as a design canvas or a board of diagrams: the app lays its content out at its own size on the plane, and the view shows it through a camera that moves and scales it on the platform's frame clock. The view owns the camera and its gestures; what the content holds and what pressing it means stay the app's. Content receives input where the camera draws it; the app also hears presses on the plane at points of the content.
package camera

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

const (
	defaultMinZoom   = .1
	defaultMaxZoom   = 4
	defaultFitMargin = 32
	zoomStep         = 1.25
	// wheelZoomDistance and pinchZoomDistance are the scroll distances in
	// points that zoom by ×1.5: a wheel's notches, and a touchpad's scroll
	// with Control or Command held.
	wheelZoomDistance = 120
	pinchZoomDistance = 48
)

// spring eases the camera toward where it goes in about a tenth of a
// second, keeping its speed when it is sent somewhere else.
var spring = core.SpringConfig{Stiffness: 666, Damping: 71, Mass: 1}

// View is the plane: it fills what the app sizes, lays Content out at its
// own size with its top left corner at the plane's origin, and shows the
// whole of it centered, no larger than its size, until the user pans or
// zooms. Drag to pan, scroll with two fingers on a touchpad to pan, turn
// the wheel or pinch to zoom around the pointer, and double-click to zoom
// in, or out with Shift, where OnPress takes no press.
type View struct {
	Content core.Node
	// State holds the camera between mounts and moves it; nil keeps one of
	// the view's own.
	State *State
	// OnPress hears a press on the plane at a point of the content, and
	// says it took it. OnHover hears the pointer move over the plane at a
	// point of the content, unless a button is down, and over false as it
	// leaves.
	OnPress func(at core.Point, e core.PressEvent) bool
	OnHover func(at core.Point, over bool)
	// Cursor is the pointer's look over the plane while it does not pan,
	// the grab hand when nil or CursorAuto.
	Cursor func() core.Cursor
	// Children sit over the plane, unmoved by the camera, such as its
	// tools, and take the pointer before it.
	Children []core.Node
	// MinZoom and MaxZoom bound the zoom, .1 and 4 when 0; FitMargin keeps
	// what a fit shows from the view's edges, 32 points when 0.
	MinZoom, MaxZoom float64
	FitMargin        core.Points
	// Label describes the plane for assistive technology. TestID names
	// the plane, and with -pointer after it the layer taking the pointer.
	Label     string
	LabelFunc func() string
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
}

// State is a camera: it keeps where it is between mounts and moves the
// camera of the view showing it. Its zero value is ready to use; keep one
// per plane. Its methods run on the UI's goroutine; those moving the
// camera do nothing while no view shows it.
type State struct {
	v     *view
	cam   camera
	moved bool
}

// camera shows the content's point p at x + p·zoom in the view.
type camera struct{ x, y, zoom float64 }

// Fit shows the whole content centered, no larger than its size, and
// keeps it fitted as it and the view change size until the user moves
// the camera.
func (s *State) Fit() {
	if s.v != nil {
		s.v.fit(true)
	}
}

// FitBounds shows a part of the content, in its coordinates, centered and
// no larger than its size, as the user moving there would.
func (s *State) FitBounds(b core.Bounds) {
	if s.v != nil {
		s.v.fitBounds(b, true)
	}
}

// Center pans to center a part of the content, in its coordinates, at
// the zoom shown, and Reveal does unless it shows whole already.
func (s *State) Center(b core.Bounds) {
	if s.v != nil {
		s.v.center(b)
	}
}

func (s *State) Reveal(b core.Bounds) {
	if s.v != nil {
		s.v.reveal(b)
	}
}

// ZoomIn and ZoomOut scale the view around its center.
func (s *State) ZoomIn() {
	if s.v != nil {
		s.v.zoomAt(s.v.width/2, s.v.height/2, zoomStep, true)
	}
}

func (s *State) ZoomOut() {
	if s.v != nil {
		s.v.zoomAt(s.v.width/2, s.v.height/2, 1/zoomStep, true)
	}
}

// Moved says the user moved the camera since the content was last
// fitted.
func (s *State) Moved() bool { return s.moved }

// Camera returns where the camera puts the content's origin in the view
// and its scale, where it is going, which a caller reading it follows
// while a view shows it.
func (s *State) Camera() (x, y, zoom float64) {
	c := s.cam
	if s.v != nil {
		c = s.v.going.Get()
	}
	if c.zoom == 0 {
		c.zoom = 1
	}
	return c.x, c.y, c.zoom
}

// Zoom is the camera's scale, where it is going, which a caller reading
// it follows, such as to keep a mark's size on screen at any zoom.
func (s *State) Zoom() float64 {
	_, _, zoom := s.Camera()
	return zoom
}

// ToView is the point of the view showing a point of the content, where
// the camera is going, and ToContent the point of the content under a
// point of the view.
func (s *State) ToView(p core.Point) core.Point {
	x, y, zoom := s.Camera()
	return core.Point{X: core.Points(x + float64(p.X)*zoom), Y: core.Points(y + float64(p.Y)*zoom)}
}

func (s *State) ToContent(p core.Point) core.Point {
	c := s.cam
	if c.zoom == 0 {
		c.zoom = 1
	}
	return core.Point{X: core.Points((float64(p.X) - c.x) / c.zoom), Y: core.Points((float64(p.Y) - c.y) / c.zoom)}
}

// view is a mounted plane.
type view struct {
	cx   *core.Scope
	s    *State
	spec View

	x, y, zoom *core.AnimatedValue
	// going is where the camera goes, for what is drawn at a point of the
	// content.
	going    *core.Signal[camera]
	run      *core.AnimationRun
	panning  *core.Signal[bool]
	shown    *core.Signal[bool]
	pinch    float64
	pressed  bool
	width    float64
	height   float64
	contentW float64
	contentH float64
}

// Render implements core.Node.
func (spec View) Render(cx *core.Scope) core.Node {
	s := spec.State
	if s == nil {
		s = &State{}
	}
	if s.cam.zoom == 0 {
		s.cam.zoom = 1
	}
	v := &view{cx: cx, s: s, spec: spec, x: core.NewAnimatedValue(cx, s.cam.x), y: core.NewAnimatedValue(cx, s.cam.y),
		zoom: core.NewAnimatedValue(cx, s.cam.zoom), going: core.NewSignal(cx, s.cam), panning: core.NewSignal(cx, false),
		shown: core.NewSignal(cx, false)}
	s.v = v
	cx.Cleanup(func() {
		if s.v == v {
			s.v = nil
		}
	})
	content := spec.Content
	if content == nil {
		content = core.Group{}
	}
	plane := core.View{PointerEvents: core.PointerEventsBoxNone, OnLayout: v.measured,
		StyleFunc: func() core.Style {
			st := core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0),
				TransformOrigin: core.ObjectPosition{X: core.PositionAxis{Offset: core.Pt(0)}, Y: core.PositionAxis{Offset: core.Pt(0)}},
				Animated:        &core.AnimatedStyle{TranslateX: v.x, TranslateY: v.y, Scale: v.zoom}}
			if !v.shown.Get() {
				// The content shows once it is placed.
				st.Opacity = core.Alpha(0)
			}
			return st
		},
		Children: []core.Node{content}}
	children := append([]core.Node{v.pointerLayer(plane)}, spec.Children...)
	return core.View{Role: core.RoleGroup, Label: spec.Label, LabelFunc: spec.LabelFunc, Class: spec.Class, TestID: spec.TestID,
		Style:     core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0), Overflow: core.Hidden}.Merge(spec.Style),
		StyleFunc: spec.StyleFunc,
		OnLayout:  func(b core.Bounds) { v.resize(float64(b.Size.Width), float64(b.Size.Height)) },
		Children:  children}
}

func (v *view) reduceMotion() bool { return core.Untracked(v.cx, v.cx.Env().ReduceMotion) }

func (v *view) zoomLimits() (lo, hi float64) {
	lo, hi = v.spec.MinZoom, v.spec.MaxZoom
	if lo <= 0 {
		lo = defaultMinZoom
	}
	if hi <= 0 {
		hi = defaultMaxZoom
	}
	return lo, hi
}

func (v *view) margin() float64 {
	if v.spec.FitMargin > 0 {
		return float64(v.spec.FitMargin)
	}
	return defaultFitMargin
}

// move sends the camera to c, at once or easing there on the platform.
func (v *view) move(c camera, animated bool) {
	v.s.cam = c
	v.going.Set(c)
	if v.run != nil {
		v.run.Stop()
	}
	if !animated || v.reduceMotion() {
		v.x.Set(c.x)
		v.y.Set(c.y)
		v.zoom.Set(c.zoom)
		return
	}
	v.run = v.cx.StartAnimation(core.Parallel(core.Spring(v.x, c.x, spring), core.Spring(v.y, c.y, spring), core.Spring(v.zoom, c.zoom, spring)), nil)
}

// fit shows the whole content and keeps it fitted.
func (v *view) fit(animated bool) {
	v.fitBounds(core.Bounds{Size: core.Size{Width: core.Points(v.contentW), Height: core.Points(v.contentH)}}, animated)
	v.s.moved = false
}

// fitBounds shows b centered, no larger than its size, as the user moving
// there would.
func (v *view) fitBounds(b core.Bounds, animated bool) {
	w, h := float64(b.Size.Width), float64(b.Size.Height)
	if v.width == 0 || w <= 0 || h <= 0 {
		return
	}
	lo, _ := v.zoomLimits()
	m := v.margin()
	zoom := clamp(min(max(1, v.width-2*m)/w, max(1, v.height-2*m)/h), lo, 1)
	v.s.moved = true
	v.move(camera{v.width/2 - (float64(b.Origin.X)+w/2)*zoom, v.height/2 - (float64(b.Origin.Y)+h/2)*zoom, zoom}, animated)
}

// center pans to center b at the zoom shown.
func (v *view) center(b core.Bounds) {
	if v.width == 0 {
		return
	}
	c := v.s.cam
	mx, my := float64(b.Origin.X+b.Size.Width/2), float64(b.Origin.Y+b.Size.Height/2)
	v.s.moved = true
	v.move(camera{v.width/2 - mx*c.zoom, v.height/2 - my*c.zoom, c.zoom}, true)
}

// reveal centers b unless it shows whole, clear of the view's edges.
func (v *view) reveal(b core.Bounds) {
	c := v.s.cam
	x, y := c.x+float64(b.Origin.X)*c.zoom, c.y+float64(b.Origin.Y)*c.zoom
	w, h := float64(b.Size.Width)*c.zoom, float64(b.Size.Height)*c.zoom
	const inset = 16
	if x >= inset && y >= inset && x+w <= v.width-inset && y+h <= v.height-inset {
		return
	}
	v.center(b)
}

// zoomAt scales the content around a point of the view.
func (v *view) zoomAt(x, y, factor float64, animated bool) {
	from := v.s.cam
	lo, hi := v.zoomLimits()
	zoom := clamp(from.zoom*factor, lo, hi)
	gx, gy := (x-from.x)/from.zoom, (y-from.y)/from.zoom
	v.s.moved = true
	v.move(camera{x - gx*zoom, y - gy*zoom, zoom}, animated)
}

// pan moves the camera by a translation.
func (v *view) pan(dx, dy float64) {
	c := v.s.cam
	v.s.moved = true
	v.move(camera{c.x + dx, c.y + dy, c.zoom}, false)
}

// place fits the content the first time both it and the view have a
// size, and while the user has not moved the camera; a plane shown again
// keeps its camera.
func (v *view) place() {
	if v.width == 0 || v.contentW == 0 || v.contentH == 0 {
		return
	}
	if !v.shown.GetUntracked() {
		v.shown.Set(true)
		if v.s.moved {
			v.move(v.s.cam, false)
			return
		}
		v.fit(false)
		return
	}
	if !v.s.moved {
		v.fit(false)
	}
}

// resize keeps the content fitted while the user has not moved the
// camera, and otherwise the same point at the view's center.
func (v *view) resize(width, height float64) {
	if width <= 0 || height <= 0 || width == v.width && height == v.height {
		return
	}
	dx, dy := (width-v.width)/2, (height-v.height)/2
	first := v.width == 0
	v.width, v.height = width, height
	if first || !v.s.moved || !v.shown.GetUntracked() {
		v.place()
		return
	}
	c := v.s.cam
	v.move(camera{c.x + dx, c.y + dy, c.zoom}, false)
}

// measured takes the content's size.
func (v *view) measured(b core.Bounds) {
	w, h := float64(b.Size.Width), float64(b.Size.Height)
	if w == v.contentW && h == v.contentH {
		return
	}
	v.contentW, v.contentH = w, h
	v.place()
}

// wheel pans and zooms: a touchpad's two fingers pan unless Control or
// Command is held, which zooms as a pinch does; a wheel zooms, except a
// sideways wheel, which pans across.
func (v *view) wheel(e core.WheelEvent) {
	modified := e.Modifiers.Control || e.Modifiers.Platform
	switch {
	case e.Phase == core.GestureEnded || e.Phase == core.GestureCancelled:
		v.glide(float64(e.Velocity.X), float64(e.Velocity.Y))
	case e.Touchpad && !modified:
		v.pan(float64(e.Delta.X), float64(e.Delta.Y))
	case !e.Touchpad && !modified && e.Delta.Y == 0:
		v.pan(float64(e.Delta.X), 0)
	default:
		distance := float64(wheelZoomDistance)
		if e.Touchpad {
			distance = pinchZoomDistance
		}
		v.zoomAt(float64(e.Position.X), float64(e.Position.Y), math.Exp(float64(e.Delta.Y)*math.Log(1.5)/distance), false)
	}
}

// glide carries a pan let go at a velocity on, slowing as a scroll view's
// glide does.
func (v *view) glide(vx, vy float64) {
	if v.reduceMotion() || math.Hypot(vx, vy) < 1 {
		return
	}
	c := v.s.cam
	if v.run != nil {
		v.run.Stop()
	}
	x, y := core.DecayConfig{Velocity: vx}, core.DecayConfig{Velocity: vy}
	v.s.cam = camera{core.ProjectDecay(c.x, x), core.ProjectDecay(c.y, y), c.zoom}
	v.going.Set(v.s.cam)
	v.run = v.cx.StartAnimation(core.Parallel(core.Decay(v.x, x), core.Decay(v.y, y)), nil)
}

// pointerLayer surrounds the content: it pans when dragged, zooms on pinches and the wheel, and tells the app of presses and hovers at points of the content.
func (v *view) pointerLayer(content core.Node) core.Node {
	press := func(e core.PressEvent) {
		if v.spec.OnPress != nil && v.spec.OnPress(v.s.ToContent(e.Position), e) {
			v.pressed = false
			return
		}
		again := v.pressed
		v.pressed = true
		if e.Clicks >= 2 && again {
			factor := 2.0
			if e.Modifiers.Shift {
				factor = .5
			}
			v.zoomAt(float64(e.Position.X), float64(e.Position.Y), factor, true)
		}
	}
	testID := ""
	if v.spec.TestID != "" {
		testID = v.spec.TestID + "-pointer"
	}
	return core.View{NoFocus: true, ImportantForAccessibility: core.ImportantNo, TestID: testID,
		Children: []core.Node{content},
		OnPress:  press,
		OnMouseMove: func(e core.MouseEvent) {
			if v.spec.OnHover != nil && e.PressedButton == core.MouseButtonNone {
				v.spec.OnHover(v.s.ToContent(e.Position), true)
			}
		},
		OnMouseLeave: func(e core.MouseEvent) {
			if v.spec.OnHover != nil {
				v.spec.OnHover(v.s.ToContent(e.Position), false)
			}
		},
		PanValues: core.AnimatedXY{X: v.x, Y: v.y},
		OnPan: func(e core.PanEvent) {
			switch e.Phase {
			case core.GestureBegan:
				if v.run != nil {
					v.run.Stop()
				}
				v.panning.Set(true)
				v.s.moved = true
			case core.GestureEnded, core.GestureCancelled:
				v.panning.Set(false)
				v.s.cam.x, v.s.cam.y = float64(e.Values.X), float64(e.Values.Y)
				v.going.Set(v.s.cam)
				// A finger let go glides on; a mouse stops where it is.
				if !core.Untracked(v.cx, v.cx.Env().Desktop) {
					v.glide(float64(e.Velocity.X), float64(e.Velocity.Y))
				}
			}
		},
		OnPinch: func(e core.GestureEvent) {
			switch e.Phase {
			case core.GestureBegan:
				v.pinch = 1
			case core.GestureChanged:
				if v.pinch > 0 && e.Value > 0 {
					v.zoomAt(float64(e.Position.X), float64(e.Position.Y), float64(e.Value)/v.pinch, false)
					v.pinch = float64(e.Value)
				}
			}
		},
		OnWheel: v.wheel,
		StyleFunc: func() core.Style {
			s := core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Cursor: core.CursorGrab}
			switch {
			case v.panning.Get():
				s.Cursor = core.CursorGrabbing
			case v.spec.Cursor != nil:
				if c := v.spec.Cursor(); c != 0 {
					s.Cursor = c
				}
			}
			return s
		}}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(v, hi)) }
