package ui

import (
	"fmt"
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// ringSize is a ProgressRing's and a Spinner's diameter by default.
const ringSize core.Points = 32

// ProgressRing is Progress as a ring: the accent, or Tint, runs clockwise from twelve o'clock over a track, or, Indeterminate, a quarter of it turns once every 1.4 s, still under reduced motion. Value is from 0 to 100, as Progress's. Size is its diameter, 32 pt when 0; the line is a tenth of it, 3 pt at most.
type ProgressRing struct {
	Value         func() float32
	Indeterminate func() bool
	Size          core.Points
	// Tint colors the arc, the accent when nil, and the track under it, a quarter as opaque, where the theme's line would not show, such as on an accent fill.
	Tint func() core.Color
	// Label names the task for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (p ProgressRing) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	tint := p.Tint
	if tint == nil {
		tint = colors.Accent
	}
	size := p.Size
	if size <= 0 {
		size = ringSize
	}
	value := func() float32 {
		if p.Value == nil {
			return 0
		}
		v := p.Value()
		if v != v {
			return 0
		}
		return min(max(v, 0), 100)
	}
	turn := core.NewAnimatedValue(cx, 0)
	if p.Indeterminate != nil && !ReduceMotion(cx) {
		cx.StartAnimation(core.ValueKeyframes(turn, core.KeyframeTiming{Duration: 1400 * time.Millisecond, Iterations: core.Forever},
			core.Stops(0, 360)...), nil)
	}
	draw := func(d *core.Drawing, s core.Size) {
		c := colors.Now()
		dia := float32(min(s.Width, s.Height))
		if dia <= 0 {
			return
		}
		width := min(dia/10, 3)
		r := (dia - width) / 2
		x, y := float32(s.Width)/2, float32(s.Height)/2
		d.SetLineWidth(width).SetLineCap(core.CapRound)
		arc, track := tint(), c.Line
		if p.Tint != nil {
			// A track of the tint's own, as the theme's line can vanish on what the tint is chosen for.
			track = Alpha(arc, 0.25)
		}
		d.BeginPath().Arc(x, y, r, 0, 2*math.Pi, false).SetStrokeColor(track).Stroke()
		start, sweep := float32(-math.Pi/2), value()/100*2*math.Pi
		if is(p.Indeterminate) {
			sweep = math.Pi / 2
		}
		if sweep <= 0 {
			return
		}
		d.BeginPath().Arc(x, y, r, start, start+sweep, false).SetStrokeColor(arc).Stroke()
	}
	// The ring is drawn; base's bar is its semantics.
	return base.Progress{TestID: p.TestID, Label: p.Label, Value: value, Indeterminate: p.Indeterminate,
		ValueText:  func(v float32) string { return fmt.Sprintf("%d%%", int(v+0.5)) },
		TrackStyle: core.Style{Display: core.DisplayNone},
		Style:      core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0}.Merge(p.Style),
		StyleFunc: func() core.Style {
			if !is(p.Indeterminate) {
				return core.Style{}
			}
			return core.Style{Animated: &core.AnimatedStyle{Rotate: turn}}
		},
		Children: []core.Node{core.Canvas{Draw: draw, Style: core.Style{FlexGrow: 1}}}}
}
