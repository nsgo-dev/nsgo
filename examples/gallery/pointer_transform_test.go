package main

import (
	"math"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// This same test runs in the model and through a target's native input.
// Aim through the root so selecting the child cannot bypass hit testing.
func TestPointersFollowNestedTransforms(t *testing.T) {
	var presses []core.PressEvent
	var hovered, panned bool
	var moved core.Point
	var backdrop int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{TestID: "root", Style: core.Style{Width: core.Pt(400), Height: core.Pt(400)},
			OnPress: func(core.PressEvent) { backdrop++ }, Children: []core.Node{
				core.View{PointerEvents: core.PointerEventsBoxNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(100), Top: core.Pt(100),
					Width: core.Pt(40), Height: core.Pt(40), Transform: core.Transform{Rotate: 90, Scale: 2}}, Children: []core.Node{
					core.View{TestID: "button", Style: core.Style{Position: core.Absolute, Left: core.Pt(5), Top: core.Pt(5),
						Width: core.Pt(20), Height: core.Pt(20), Transform: core.Transform{TranslateY: -100}, Cursor: core.CursorGrab},
						OnPress:      func(e core.PressEvent) { presses = append(presses, e) },
						OnMouseEnter: func(core.MouseEvent) { hovered = true },
						OnMouseMove:  func(e core.MouseEvent) { moved = e.Position },
						OnPan:        func(e core.PanEvent) { panned = panned || e.Phase == core.GestureBegan }},
				}},
			}}
	})
	app.PressAt("id:root", core.Point{X: 330, Y: 110})
	if len(presses) != 1 || math.Abs(float64(presses[0].Position.X-10)) > .01 || math.Abs(float64(presses[0].Position.Y-10)) > .01 {
		t.Fatalf("drawn button press = %+v, want local (10,10)", presses)
	}
	app.PressAt("id:root", core.Point{X: 115, Y: 115})
	if len(presses) != 1 || backdrop != 1 {
		t.Fatalf("old position still presses button: presses=%+v, backdrop=%d", presses, backdrop)
	}
	app.HoverAt("id:root", core.Point{X: 330, Y: 110})
	if !hovered {
		t.Error("drawn button did not receive hover")
	}
	app.HoverAt("id:button", core.Point{X: 4, Y: 8})
	if math.Abs(float64(moved.X-4)) > .01 || math.Abs(float64(moved.Y-8)) > .01 {
		t.Errorf("hover local = %+v, want (4,8)", moved)
	}
	app.DragAt("id:root", core.Point{X: 330, Y: 110}, core.Point{X: 30})
	if !panned {
		t.Error("drag from drawn button did not begin its pan")
	}
}

func TestPointersFollowTransformsAfterScrollAndStyleChanges(t *testing.T) {
	var presses []core.PressEvent
	var backdrop int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shift := core.NewSignal(cx, core.Points(-60))
		return core.View{TestID: "root", Style: core.Style{Width: core.Pt(400), Height: core.Pt(400)},
			OnPress: func(core.PressEvent) { backdrop++ }, Children: []core.Node{
				core.Scroll{TestID: "scroll", HideIndicator: true, ContentOffset: core.Point{Y: 80},
					Style: core.Style{Position: core.Absolute, Left: core.Pt(40), Top: core.Pt(40),
						Width: core.Pt(200), Height: core.Pt(200), Transform: core.Transform{Scale: 1.5}},
					Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(500)}, Children: []core.Node{
						core.View{TestID: "button", StyleFunc: func() core.Style {
							return core.Style{Position: core.Absolute, Left: core.Pt(60), Top: core.Pt(180),
								Width: core.Pt(20), Height: core.Pt(20), Transform: core.Transform{TranslateY: shift.Get()}}
						}, OnPress: func(e core.PressEvent) { presses = append(presses, e) }},
					}}}},
				core.View{TestID: "move", Style: core.Style{Position: core.Absolute, Left: core.Pt(300),
					Top: core.Pt(10), Width: core.Pt(80), Height: core.Pt(40)}, OnPress: func(core.PressEvent) { shift.Set(-20) }},
				core.View{TestID: "clip", Style: core.Style{Position: core.Absolute, Left: core.Pt(300),
					Top: core.Pt(60), Width: core.Pt(80), Height: core.Pt(40)}, OnPress: func(core.PressEvent) { shift.Set(140) }},
			}}
	})
	app.PressAt("id:root", core.Point{X: 95, Y: 65})
	if len(presses) != 1 || math.Abs(float64(presses[0].Position.X-10)) > .01 || math.Abs(float64(presses[0].Position.Y-10)) > .01 {
		t.Fatalf("scrolled button press = %+v, want local (10,10)", presses)
	}
	app.Press("id:move")
	app.WaitUntil(time.Second, func() bool { return app.Find("id:button").Style.Transform.TranslateY == -20 })
	app.PressAt("id:root", core.Point{X: 95, Y: 65})
	app.PressAt("id:root", core.Point{X: 95, Y: 125})
	if len(presses) != 2 || backdrop != 1 {
		t.Fatalf("updated transform: presses=%+v, backdrop=%d", presses, backdrop)
	}
	// The child now draws below the scroll viewport, still inside root.
	app.Press("id:clip")
	app.WaitUntil(time.Second, func() bool { return app.Find("id:button").Style.Transform.TranslateY == 140 })
	app.PressAt("id:root", core.Point{X: 95, Y: 365})
	if len(presses) != 2 || backdrop != 2 {
		t.Fatalf("clipped transform: presses=%+v, backdrop=%d", presses, backdrop)
	}
}
