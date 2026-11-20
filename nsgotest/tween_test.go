package nsgotest

import (
	"fmt"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

func TestTweenUpdatesAValueAndSettlesInAStep(t *testing.T) {
	var value *core.Signal[float64]
	var ended []bool
	app := Start(t, func(cx *core.Scope) core.Node {
		value = core.NewSignal(cx, float64(0))
		cx.Tween(0, 100, core.TimingConfig{Duration: 100 * time.Millisecond}, value.Set, func(finished bool) { ended = append(ended, finished) })
		return core.Text{TestID: "value", ContentFunc: func() string { return fmt.Sprintf("%.0f", value.Get()) }}
	})
	if got := app.Find("id:value").Text; got != "100" {
		t.Fatalf("tween settled at %q, want 100", got)
	}
	if len(ended) != 1 || !ended[0] {
		t.Fatalf("done got %v, want [true]", ended)
	}
}

func TestTweenRespectsReducedMotion(t *testing.T) {
	var updates []float64
	var run *core.AnimationRun
	Start(t, func(cx *core.Scope) core.Node {
		run = cx.Tween(2, 8, core.TimingConfig{Duration: time.Second}, func(value float64) { updates = append(updates, value) }, nil)
		return core.Text{Content: "done"}
	}, Environment(core.Environment{ReduceMotion: true}))
	if len(updates) != 2 || updates[0] != 2 || updates[1] != 8 {
		t.Fatalf("reduced-motion updates = %v, want [2 8]", updates)
	}
	run.Stop()
	run.Reset()
	if len(updates) != 3 || updates[2] != 2 {
		t.Fatalf("after Stop and Reset updates = %v, want [2 8 2]", updates)
	}
}

// A tween waits out its delay, moves on the frames between, and its run's Stop leaves the number where it is, Reset puts it back at from.
func TestTweenFollowsTheClockAndStopsWhereItIs(t *testing.T) {
	var value *core.Signal[float64]
	var run *core.AnimationRun
	app := Start(t, func(cx *core.Scope) core.Node {
		value = core.NewSignal(cx, float64(0))
		run = cx.Tween(0, 100, core.TimingConfig{Duration: 400 * time.Millisecond, Delay: 100 * time.Millisecond}, value.Set, nil)
		return core.Text{Content: "tween"}
	}, ManualFrames())
	app.Advance(80 * time.Millisecond)
	if v := value.Get(); v != 0 {
		t.Fatalf("the tween moved to %v within its delay", v)
	}
	app.Advance(220 * time.Millisecond)
	mid := value.Get()
	if mid <= 0 || mid >= 100 {
		t.Fatalf("the tween is at %v halfway, want between 0 and 100", mid)
	}
	run.Stop() // headless, the test goroutine is the graph's
	app.Advance(time.Second)
	if v := value.Get(); v != mid {
		t.Fatalf("a stopped tween moved from %v to %v", mid, v)
	}
	run.Reset()
	app.Advance(time.Second)
	if v := value.Get(); v != 0 {
		t.Fatalf("a reset tween is at %v, want 0", v)
	}
}
