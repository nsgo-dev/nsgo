package canvasrecording

import (
	"image/color"
	"testing"
	"time"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestCanvasRecordingPixelsAndReplacement(t *testing.T) {
	red, blue, green := core.RGB(0xe23421), core.RGB(0x253bdd), core.RGB(0x18c653)
	const width, height = 130, 40
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		makeLayer := func(right core.Color) *core.ImageSource {
			tile := core.RecordCanvasImage(cx, core.Size{Width: 17, Height: 19}, 2.5, func(d *core.Drawing) {
				d.SetFillColor(right).FillRect(0, 0, 17, 19)
			})
			layer := core.RecordCanvasImage(cx, core.Size{Width: 31, Height: 19}, 2.5, func(d *core.Drawing) {
				d.SetFillColor(red).FillRect(0, 0, 14, 19)
				d.DrawImage(tile, 14, 0)
			})
			return &layer
		}
		phase := core.NewSignal(cx, makeLayer(blue))
		return core.View{TestID: "root", Style: core.Style{Width: core.Pt(width), Height: core.Pt(height)},
			OnPress: func(core.PressEvent) { phase.Set(makeLayer(green)) },
			Children: []core.Node{core.Canvas{
				TestID: "recording",
				Style:  core.Style{Width: core.Pt(width), Height: core.Pt(height)},
				Draw: func(d *core.Drawing, _ core.Size) {
					src := *phase.Get()
					d.DrawImage(src, 0, 0)
					d.DrawImageFrom(src, 14, 0, 17, 19, 42, 0, 17, 19)
					d.SetFillColor(red).FillRect(70, 0, 15, 19)
					d.SetFillColor(blue).FillRect(90, 0, 15, 19)
					d.SetFillColor(green).FillRect(110, 0, 15, 19)
				},
			}},
		}
	})
	a.WaitFor("id:recording", 5*time.Second)
	match := func(rightX int) bool {
		img := a.Capture()
		root := a.Find("id:root")
		box := root.Bounds()
		ratio := float64(img.Bounds().Dx()) / float64(root.Parent.Frame.Size.Width)
		at := func(x int) color.RGBA {
			capture := img.Bounds()
			px := capture.Min.X + int((float64(box.Origin.X)+float64(x))*ratio)
			py := capture.Min.Y + int((float64(box.Origin.Y)+8)*ratio)
			return color.RGBAModel.Convert(img.At(px, py)).(color.RGBA)
		}
		first, second, crop := at(5), at(22), at(49)
		return first == at(75) && first != second && second == at(rightX) && crop == second
	}
	a.WaitUntil(5*time.Second, func() bool { return match(97) })
	a.Press("id:root")
	a.WaitUntil(5*time.Second, func() bool { return match(117) })
}
