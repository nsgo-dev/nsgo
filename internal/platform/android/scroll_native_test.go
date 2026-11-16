//go:build android && nsgotest

package android

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// The agent dispatches real MotionEvents through the Activity. These tests exercise the native scroll view and consuming, recycled rows, rather than nsgotest's model of a touch.
func listTouchStopsMomentum(t *testing.T, horizontal bool) {
	var downs, dragStarts, dragEnds, starts, ends, rests atomic.Int32
	started := make(chan struct{}, 1)
	surface, err := testhost.Current().Open(func(cx *core.Scope) core.Node {
		return core.List{
			Count: func() int { return 2000 }, Horizontal: horizontal,
			Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)},
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				style := core.Style{Width: core.Pt(200)}
				if horizontal {
					style.Width, style.Height = core.Pt(100), core.Pt(200)
				}
				return core.View{
					Style: style, Role: core.RoleButton,
					OnPress:     func(core.PressEvent) {},
					OnPressDown: func(core.PressEvent) { downs.Add(1) },
					Children: []core.Node{core.Text{ContentFunc: func() string {
						return fmt.Sprintf("Row %d: %s", row.Index(), strings.Repeat("variable height text ", 2+row.Index()%5))
					}}},
				}
			},
			OnScrollDragStart: func(core.ScrollEvent) { dragStarts.Add(1) },
			OnScrollDragEnd:   func(core.ScrollEvent) { dragEnds.Add(1) },
			OnMomentumStart: func(core.ScrollEvent) {
				starts.Add(1)
				select {
				case started <- struct{}{}:
				default:
				}
			},
			OnMomentumEnd: func(core.ScrollEvent) { ends.Add(1) },
			OnScrollEnd:   func(core.ScrollEvent) { rests.Add(1) },
		}
	}, testhost.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := surface.(*testSurface)
	from, to := driver.Point{X: 100, Y: 400}, driver.Point{X: 100, Y: 150}
	if horizontal {
		from = driver.Point{X: 300, Y: 150}
	}
	// Wait for scrollable native content, rather than sleeping through layout.
	deadline := time.Now().Add(5 * time.Second)
	for {
		ready, err := s.ask(map[string]any{"op": "scroll-ready", "x": from.X, "y": from.Y})
		if err != nil {
			t.Fatal(err)
		}
		if ready == "true" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the native list did not become scrollable")
		}
		time.Sleep(20 * time.Millisecond)
	}
	blank := driver.Point{X: 300, Y: 150}
	if horizontal {
		blank = driver.Point{X: 100, Y: 300}
	}
	for trial, at := range []driver.Point{to, blank} {
		if err := s.do(map[string]any{"op": "swipe", "path": []driver.Point{from, to}}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("the native swipe did not start momentum")
		}
		if ends.Load() != int32(trial) {
			t.Fatal("momentum ended before the braking touch")
		}
		before := downs.Load()
		drift, err := s.ask(map[string]any{"op": "hold-scroll", "x": at.X, "y": at.Y})
		if err != nil {
			t.Fatal(err)
		}
		pixels, err := strconv.Atoi(drift)
		if err != nil {
			t.Fatal(err)
		}
		if pixels != 0 {
			t.Errorf("scroll moved %d native pixels over 20 frames while a child held the finger", pixels)
		}
		s.Sync()
		wantDowns := before
		if trial == 0 {
			wantDowns++
		}
		if downs.Load() != wantDowns {
			t.Errorf("braking DOWN count %d; want %d (trial %d)", downs.Load(), wantDowns, trial)
		}
		if starts.Load() != int32(trial+1) || ends.Load() != int32(trial+1) {
			t.Errorf("momentum events: start %d, end %d; want %d each", starts.Load(), ends.Load(), trial+1)
		}
		if dragStarts.Load() != int32(trial+1) || dragEnds.Load() != int32(trial+1) {
			t.Errorf("drag events: start %d, end %d; braking touch must not create a drag", dragStarts.Load(), dragEnds.Load())
		}
		if rests.Load() != 0 {
			t.Errorf("a touch interrupting momentum reported %d scroll ends", rests.Load())
		}
	}
}
