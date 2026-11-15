package diagram

import (
	"sync"

	"github.com/firede/limu/text"

	"github.com/nsgo-dev/nsgo/core"
)

// measurer measures labels for limu's layout with the platform's text engine, in the fonts paint draws them in, so text fills the room the layout gave it. It keeps what it measured: a diagram's layout measures the same words many times over.
type measurer struct {
	cx *core.Scope
	mu sync.Mutex
	m  map[measureKey]float64
}

type measureKey struct {
	s    string
	size float64
	face text.Face
}

// maxMeasured is how many widths a measurer keeps before it lets them all go.
const maxMeasured = 8192

func newMeasurer(cx *core.Scope) *measurer { return &measurer{cx: cx, m: map[measureKey]float64{}} }

// Width implements text.Measurer: how wide s is set in face f at size, which limu has already made smaller for a Sub or Sup span.
func (m *measurer) Width(s string, size float64, f text.Face) float64 {
	key := measureKey{s: s, size: size, face: f}
	m.mu.Lock()
	w, ok := m.m[key]
	m.mu.Unlock()
	if ok {
		return w
	}
	w = float64(m.cx.MeasureText(s, font(size, f)).Width)
	m.mu.Lock()
	if len(m.m) >= maxMeasured {
		clear(m.m)
	}
	m.m[key] = w
	m.mu.Unlock()
	return w
}

// font is the platform's font for a span of face f at size: the system font, or its monospaced one, in bold or italics as f says.
func font(size float64, f text.Face) core.Font {
	ft := core.Font{Size: core.Points(size)}
	if f&text.Mono != 0 {
		ft.Family = core.FontMonospace
	}
	if f&text.Bold != 0 {
		ft.Weight = core.FontWeightBold
	}
	ft.Italic = f&text.Italic != 0
	return ft
}
