//go:build js && wasm

package harmonyos

import (
	"encoding/base64"
	"encoding/json"
	"weak"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/webstyle"
)

// canvasOp is one drawing operation as the host replays it into a
// CanvasRenderingContext2D. Short keys and omitted zero fields keep a
// drawing small.
type canvasOp struct {
	K    int       `json:"k"`
	X    float32   `json:"x,omitempty"`
	Y    float32   `json:"y,omitempty"`
	X1   float32   `json:"x1,omitempty"`
	Y1   float32   `json:"y1,omitempty"`
	X2   float32   `json:"x2,omitempty"`
	Y2   float32   `json:"y2,omitempty"`
	W    float32   `json:"w,omitempty"`
	H    float32   `json:"h,omitempty"`
	R    float32   `json:"r,omitempty"`
	A    float32   `json:"a,omitempty"`
	A0   float32   `json:"a0,omitempty"`
	A1   float32   `json:"a1,omitempty"`
	RY   float32   `json:"ry,omitempty"`
	CCW  bool      `json:"ccw,omitempty"`
	C    string    `json:"c,omitempty"`
	Cap  int       `json:"cap,omitempty"`
	Join int       `json:"join,omitempty"`
	EO   bool      `json:"eo,omitempty"`
	T    string    `json:"t,omitempty"`
	FF   string    `json:"ff,omitempty"`
	FS   float32   `json:"fs,omitempty"`
	FW   int       `json:"fw,omitempty"`
	FI   bool      `json:"fi,omitempty"`
	G    *gradient `json:"g,omitempty"`
	CO   int       `json:"co,omitempty"`
	D    []float32 `json:"d,omitempty"`
	Img  string    `json:"img,omitempty"`
	// ImgData is the image's bytes, base64, when the drawing embeds them.
	ImgData   string `json:"imgd,omitempty"`
	Recording string `json:"recording,omitempty"`
}

// gradient is a canvas gradient as the host builds one: radial or linear,
// its circles or endpoints, its stops.
type gradient struct {
	Radial bool    `json:"radial,omitempty"`
	X1     float32 `json:"x1,omitempty"`
	Y1     float32 `json:"y1,omitempty"`
	R1     float32 `json:"r1,omitempty"`
	X2     float32 `json:"x2,omitempty"`
	Y2     float32 `json:"y2,omitempty"`
	R2     float32 `json:"r2,omitempty"`
	Stops  []stop  `json:"s,omitempty"`
}

type stop struct {
	O float32 `json:"o"`
	C string  `json:"c"`
}

// canvasJSON serializes a drawing for the host.
func canvasJSON(ops []driver.DrawOp) string {
	out := make([]canvasOp, 0, len(ops))
	for i := range ops {
		o := &ops[i]
		c := canvasOp{
			K: int(o.Kind), X: o.X, Y: o.Y, X1: o.X1, Y1: o.Y1, X2: o.X2, Y2: o.Y2,
			W: o.W, H: o.H, R: o.R, A: o.A, A0: o.A0, A1: o.A1, RY: o.RY, CCW: o.CCW, EO: o.EvenOdd,
			T: o.Text, FF: o.Font.Family, FS: float32(o.Font.Size), FW: int(o.Font.Weight), FI: o.Font.Italic,
		}
		switch o.Kind {
		case driver.DrawFillColor, driver.DrawStrokeColor, driver.DrawShadow:
			c.C = webstyle.Color(o.Color)
		case driver.DrawLineCap:
			c.Cap = int(o.Cap) + 1
		case driver.DrawLineJoin:
			c.Join = int(o.Join) + 1
		case driver.DrawComposite:
			c.CO = int(o.Composite) + 1
		case driver.DrawFillGradient, driver.DrawStrokeGradient:
			c.G = gradientOf(o.Gradient)
		case driver.DrawLineDash:
			if o.Dash != nil {
				c.D = append([]float32(nil), (*o.Dash)...)
			}
		case driver.DrawImage:
			if o.Image != nil {
				if o.Image.Recording != nil {
					c.Recording = o.Image.Key()
				}
				c.Img = o.Image.URI
				if len(o.Image.Data) > 0 {
					c.ImgData = base64.StdEncoding.EncodeToString(o.Image.Data)
				}
			}
		}
		out = append(out, c)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// gradientOf maps a drawing gradient, leaving out what the host cannot play.
func gradientOf(g *driver.CanvasGradient) *gradient {
	if g == nil || g.Conic {
		return nil
	}
	out := &gradient{Radial: g.Radial, X1: g.X1, Y1: g.Y1, R1: g.R1, X2: g.X2, Y2: g.Y2, R2: g.R2}
	for _, s := range g.Stops {
		out.Stops = append(out.Stops, stop{O: s.Offset, C: webstyle.Color(s.Color)})
	}
	return out
}

type canvasRecording struct {
	Width      driver.Points   `json:"w"`
	Height     driver.Points   `json:"h"`
	PixelsWide int             `json:"pw"`
	PixelsHigh int             `json:"ph"`
	Ops        json.RawMessage `json:"ops"`
}

func (m *mounter) canvasDrawing(id driver.NodeID, ops []driver.DrawOp) {
	m.sendCanvasSources(ops)
	m.o.op("cv", id).str(canvasJSON(ops))
}

func (m *mounter) sendCanvasSources(ops []driver.DrawOp) {
	for _, op := range ops {
		if op.Image == nil || op.Image.Recording == nil {
			continue
		}
		key, r := op.Image.Key(), op.Image.Recording
		if _, ok := m.recordings[key]; ok {
			continue
		}
		if m.recordings == nil {
			m.recordings = map[string]weak.Pointer[driver.CanvasRecording]{}
		}
		m.recordings[key] = weak.Make(r)
		m.sendCanvasSources(r.Ops)
		data, _ := json.Marshal(canvasRecording{Width: r.Size.Width, Height: r.Size.Height, PixelsWide: r.PixelWidth,
			PixelsHigh: r.PixelHeight, Ops: json.RawMessage(canvasJSON(r.Ops))})
		m.o.op("cr", 0).str(key).str(string(data))
	}
}

func (m *mounter) collectCanvasSources() {
	for key, r := range m.recordings {
		if r.Value() == nil {
			m.o.op("cd", 0).str(key)
			delete(m.recordings, key)
		}
	}
}
