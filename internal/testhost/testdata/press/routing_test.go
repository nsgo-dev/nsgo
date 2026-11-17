package press_test

import (
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestPressRoutingDoesNotDisableSiblings(t *testing.T) {
	var earlier, later int
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		button := func(id string, x core.Points, press func()) core.Node {
			return core.View{TestID: id, Style: core.Style{Position: core.Absolute,
				Left: core.Pt(x), Top: core.Pt(20), Width: core.Pt(100), Height: core.Pt(60)},
				OnPress: func(core.PressEvent) { press() }, Children: []core.Node{core.Text{Content: id}}}
		}
		return core.View{TestID: "root", Style: core.Style{Width: core.Pt(300), Height: core.Pt(200)},
			Children: []core.Node{
				button("earlier", 20, func() { earlier++ }),
				button("later", 170, func() { later++ }),
			}}
	}, nsgotest.Size(300, 200))
	for i := 1; i <= 3; i++ {
		app.PressAt("id:root", core.Point{X: 70, Y: 50})
		app.PressAt("id:root", core.Point{X: 220, Y: 50})
		if earlier != i || later != i {
			t.Fatalf("alternating press %d: earlier=%d, later=%d", i, earlier, later)
		}
	}
}

func TestOverflowPressLeavesItsContainersPressable(t *testing.T) {
	var child, parent, backdrop int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		clipped := core.NewSignal(cx, false)
		return core.View{TestID: "root", Style: core.Style{Width: core.Pt(300), Height: core.Pt(250)},
			OnPress: func(core.PressEvent) { backdrop++ }, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					overflow := core.Visible
					if clipped.Get() {
						overflow = core.Hidden
					}
					return core.Style{Position: core.Absolute, Left: core.Pt(150), Top: core.Pt(100),
						Width: core.Pt(80), Height: core.Pt(80), Overflow: overflow}
				}, OnPress: func(core.PressEvent) { parent++ }, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(20), Height: core.Pt(20)}, Children: []core.Node{
						core.View{Style: core.Style{Width: core.Pt(20), Height: core.Pt(20),
							Transform: core.Transform{TranslateX: -100}}, OnPress: func(core.PressEvent) { child++ }},
					}},
				}},
				core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(240), Top: core.Pt(200),
					Width: core.Pt(50), Height: core.Pt(40)}, OnPress: func(core.PressEvent) { clipped.Set(true) }},
			}}
	}, nsgotest.Size(300, 250))
	for i := 1; i <= 3; i++ {
		app.PressAt("id:root", core.Point{X: 60, Y: 110})
		app.PressAt("id:root", core.Point{X: 210, Y: 150})
		if child != i || parent != i || backdrop != 0 {
			t.Fatalf("overflow press %d: child=%d, parent=%d, backdrop=%d", i, child, parent, backdrop)
		}
	}
	app.PressAt("id:root", core.Point{X: 265, Y: 220})
	app.PressAt("id:root", core.Point{X: 60, Y: 110})
	if child != 3 || parent != 3 || backdrop != 1 {
		t.Fatalf("clipped press: child=%d, parent=%d, backdrop=%d", child, parent, backdrop)
	}
}

func TestPressRoutingUsesNestedTransforms(t *testing.T) {
	var presses []core.PressEvent
	var backdrop int
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return core.View{TestID: "root", Style: core.Style{Width: core.Pt(400), Height: core.Pt(400)},
			OnPress: func(core.PressEvent) { backdrop++ }, Children: []core.Node{
				core.View{PointerEvents: core.PointerEventsBoxNone, Style: core.Style{Position: core.Absolute,
					Left: core.Pt(100), Top: core.Pt(100), Width: core.Pt(40), Height: core.Pt(40),
					Transform: core.Transform{Rotate: 90, Scale: 2}}, Children: []core.Node{
					core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(5), Top: core.Pt(5),
						Width: core.Pt(20), Height: core.Pt(20), Transform: core.Transform{TranslateY: -100}},
						OnPress: func(e core.PressEvent) { presses = append(presses, e) }},
				}},
			}}
	}, nsgotest.Size(400, 400))
	app.PressAt("id:root", core.Point{X: 330, Y: 110})
	if len(presses) != 1 || math.Abs(float64(presses[0].Position.X-10)) > .5 || math.Abs(float64(presses[0].Position.Y-10)) > .5 {
		t.Fatalf("transformed press = %+v, want local (10,10)", presses)
	}
	app.PressAt("id:root", core.Point{X: 115, Y: 115})
	if len(presses) != 1 || backdrop != 1 {
		t.Fatalf("old position: presses=%+v, backdrop=%d", presses, backdrop)
	}
}
