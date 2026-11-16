package main

import (
	"math"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestSwipeVelocityUsesTheAppClock measures a swipe's speed and lets it go stale on the app's clock, which the test moves, so how fast the test runs changes neither: a swipe let go within 80 ms flings on with its speed, a later one rests on the nearest card.
func TestSwipeVelocityUsesTheAppClock(t *testing.T) {
	headless(t)
	for _, c := range []struct {
		name   string
		pause  time.Duration
		target float64
	}{
		// -120 points turn the ring -36 degrees; the speed, -4800 points a second capped to -3000, carries it 180 degrees further, to -216, whose nearest card is at -240.
		{"fresh", 50 * time.Millisecond, -4 * step},
		{"stale", 100 * time.Millisecond, -step},
	} {
		t.Run(c.name, func(t *testing.T) {
			var s swipe
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				s.r = newRing(cx)
				return core.View{}
			}, nsgotest.Environment(core.Environment{ReduceMotion: true}))
			app.Post(func() { s.wheel(core.WheelEvent{Touchpad: true, Phase: core.GestureBegan}) })
			app.Settle()
			app.Advance(10 * time.Millisecond)
			app.Post(func() {
				s.wheel(core.WheelEvent{Touchpad: true, Phase: core.GestureChanged, Delta: core.Point{X: -120}})
			})
			app.Settle()
			if math.Abs(s.speed+4800) > 0.001 {
				t.Fatalf("speed %g, want -4800 points a second after 10 ms", s.speed)
			}
			app.Advance(c.pause)
			app.Post(func() { s.wheel(core.WheelEvent{Touchpad: true, Phase: core.GestureEnded}) })
			app.Settle()
			if s.r.target != c.target {
				t.Fatalf("let go after %v, the ring rests at %g, want %g", c.pause, s.r.target, c.target)
			}
		})
	}
}
