package main

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/base/camera"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
	uiflow "github.com/nsgo-dev/nsgo/ui/flow"
)

// cameraScreen is a screen of the board: its name and the lines it shows.
type cameraScreen struct {
	id    string
	name  flowText
	lines []flowText
}

var cameraScreens = []cameraScreen{
	{id: "sign-in", name: fm.CameraSignIn, lines: []flowText{fm.CameraWelcome, fm.CameraEmail, fm.CameraContinue}},
	{id: "menu", name: fm.CameraMenu, lines: []flowText{fm.CameraLatte, fm.CameraFlatWhite, fm.CameraColdBrew}},
	{id: "cart", name: fm.CameraCart, lines: []flowText{fm.CameraLatte, fm.CameraTotal, fm.CameraPay}},
}

// cameraShown and cameraChart are the camera and the flow's state of the
// page shown last, which tests read.
var (
	cameraShown *camera.State
	cameraChart *uiflow.State
)

// cameraDemo is a board of screens and the flow between them on a camera
// the page composes itself: the board lays out at its own size, the
// camera fits, pans and zooms it, the page finds what a press is on from
// the point of the board the camera hands it and draws its toolbar over
// the plane.
func cameraDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	cam, chart := &camera.State{}, &uiflow.State{}
	cameraShown, cameraChart = cam, chart
	selected, node := core.NewSignal(cx, ""), core.NewSignal(cx, "")
	pointer := core.NewSignal(cx, "")
	// frames are where each item of the board is in it, by its ID, the
	// flow's "flow".
	frames := map[string]core.Bounds{}
	framed := core.NewSignal(cx, 0)
	frame := func(id string) func(core.Bounds) {
		return func(b core.Bounds) {
			frames[id] = b
			framed.Update(func(n int) int { return n + 1 })
		}
	}
	// item is an item of the board under its name.
	item := func(id, name string, body core.Node) core.Node {
		return core.View{TestID: "board-" + id, OnLayout: frame(id), Style: core.Style{Gap: 10}, Children: []core.Node{
			ui.Text(name, 13, ui.Semibold, colors.Muted), body}}
	}
	screen := func(s cameraScreen) core.Node {
		var lines []core.Node
		for i, l := range s.lines {
			weight := ui.Regular
			if i == 0 {
				weight = ui.Semibold
			}
			lines = append(lines, core.View{Style: core.Style{Padding: core.Symmetric(10, 12), CornerRadius: 10},
				StyleFunc: ui.Filled(colors.Inset), Children: []core.Node{ui.Text(l(tr), 13, weight, colors.Text)}})
		}
		return item(s.id, s.name(tr), core.View{Style: core.Style{Width: core.Pt(240), Height: core.Pt(420), Padding: core.All(16), Gap: 10,
			CornerRadius: 28, BorderWidth: 1, BorderCurve: core.BorderCurveContinuous},
			StyleFunc: func() core.Style {
				border := colors.Border()
				if selected.Get() == s.id {
					border = colors.Accent()
				}
				return core.Style{Background: colors.Surface(), BorderColor: border}
			},
			Children: lines})
	}
	data := uiflow.Data{
		Nodes: []uiflow.Node{
			{ID: "pick", Label: tr.CameraPick(), Content: uiflow.Card{Title: tr.CameraPick()}},
			{ID: "pay", Label: tr.CameraPay(), Content: uiflow.Card{Title: tr.CameraPay()}},
			{ID: "ready", Label: tr.CameraReady(), Content: uiflow.Card{Title: tr.CameraReady()}},
		},
		Edges: []uiflow.Edge{{Source: "pick", Target: "pay"}, {Source: "pay", Target: "ready"}, {Source: "pay", Target: "pick", Dashed: true, Label: tr.CameraDeclined()}},
	}
	// origin is where the flow's diagram is in the board, under its name.
	const nameRoom = 28
	origin := func() core.Point {
		f := frames["flow"]
		return core.Point{X: f.Origin.X, Y: f.Origin.Y + nameRoom}
	}
	board := core.View{TestID: "board", Style: core.Style{Direction: core.Row, Gap: 64, AlignItems: core.AlignStart}, Children: []core.Node{
		screen(cameraScreens[0]), screen(cameraScreens[1]), screen(cameraScreens[2]),
		item("flow", tr.CameraCheckout(), uiflow.Diagram{State: chart, Camera: cam, Origin: origin, Data: func() uiflow.Data { return data },
			Selected: node.Get, OnSelect: node.Set}),
	}}
	// at is the item of the board at a point of it.
	at := func(p core.Point) string {
		for id, b := range frames {
			if p.X >= b.Origin.X && p.Y >= b.Origin.Y && p.X < b.Origin.X+b.Size.Width && p.Y < b.Origin.Y+b.Size.Height {
				return id
			}
		}
		return ""
	}
	inFlow := func(p core.Point) core.Point {
		o := origin()
		return core.Point{X: p.X - o.X, Y: p.Y - o.Y}
	}
	press := func(p core.Point, e core.PressEvent) bool {
		id := at(p)
		cx.Batch(func() {
			if id == "flow" {
				selected.Set("")
				return
			}
			node.Set("")
			selected.Set(id)
		})
		if id == "flow" {
			return chart.Press(inFlow(p), e)
		}
		return id != ""
	}
	hover := func(p core.Point, over bool) {
		chart.Hover(inFlow(p), over && at(p) == "flow")
		if over {
			pointer.Set(fmt.Sprintf("%.0f, %.0f", p.X, p.Y))
		} else {
			pointer.Set("")
		}
	}
	tool := func(icon func() *core.SVG, label string, enabled func() bool, press func()) core.Node {
		return ui.IconButton{Icon: icon, Label: label, OnPress: press, Disabled: func() bool { return enabled != nil && !enabled() }}
	}
	toolbar := core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(8), Right: core.Pt(8), Direction: core.Row, Gap: 2,
		Padding: core.All(4), CornerRadius: ui.RadiusControl + 4, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return fin.Raised().Over(core.Style{Background: colors.Surface(), BorderColor: colors.Border()})
		},
		Children: []core.Node{
			tool(lucide.ZoomOut, tr.FlowZoomOut(), nil, cam.ZoomOut),
			tool(lucide.ZoomIn, tr.FlowZoomIn(), nil, cam.ZoomIn),
			tool(lucide.Scan, tr.FlowFit(), nil, cam.Fit),
			tool(lucide.Focus, tr.CameraFitSelection(), func() bool { return selected.Get() != "" }, func() {
				if b, ok := frames[selected.GetUntracked()]; ok {
					cam.FitBounds(b)
				}
			}),
		}}
	plane := camera.View{TestID: "camera", State: cam, Content: board, OnPress: press, OnHover: hover, Label: tr.CameraLabel(),
		Cursor: func() core.Cursor {
			if chart.Hovered() != "" {
				return core.CursorDefault
			}
			return core.CursorAuto
		},
		Children: []core.Node{toolbar}}
	return stagePage(cx,
		core.View{StyleFunc: func() core.Style {
			return fin.Card().Over(core.Style{FlexGrow: 1, FlexShrink: core.Shrink(1), MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset,
				BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Background(), Overflow: core.Hidden})
		}, Children: []core.Node{PixelGrid{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{plane}}}},
		Section{Title: tr.TerminalStatus(), Note: "camera.State", Children: []core.Node{
			statusLine(cx, tr.CameraZoom(), "camera-zoom", func() string { return fmt.Sprintf("%.0f%%", cam.Zoom()*100) }),
			statusLine(cx, tr.FlowSelected(), "camera-selected", dashIfEmpty(func() string {
				if n := node.Get(); n != "" {
					return "flow/" + n
				}
				return selected.Get()
			})),
			statusLine(cx, tr.CameraPointer(), "camera-pointer", dashIfEmpty(pointer.Get)),
		}})
}
