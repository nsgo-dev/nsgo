//go:build linux && !android

package linux

import (
	"fmt"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A config arriving after the refresh animation has ended must keep the refresh room. A fast confirming commit used to hide this race because the still-running animation restored the offset it had clamped away.
func TestRefreshingKeepsRoomAcrossCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	type state struct {
		step                   int
		top, bottom, indicator driver.Points
		height, content        driver.Points
		refreshing             bool
	}
	var root *core.Scope
	var value *core.Signal[state]
	app := func(cx *core.Scope) core.Node {
		root = cx
		value = core.NewSignal(cx, state{height: 300, content: 1000, refreshing: true})
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("refresh-commit|%d", value.Get().step)) })
		return core.Scroll{
			StyleFunc: func() core.Style {
				return core.Style{Width: driver.Pt(200), Height: driver.Pt(value.Get().height)}
			},
			BehaviorFunc: func() core.ScrollBehavior {
				s := value.Get()
				return core.ScrollBehavior{ContentInset: driver.Edges{Top: s.top, Bottom: s.bottom}, RefreshIndicator: s.indicator}
			},
			Refreshing: func() bool { return value.Get().refreshing }, OnRefresh: func() {},
			Children: []core.Node{core.View{StyleFunc: func() core.Style {
				return core.Style{Height: driver.Pt(value.Get().content)}
			}}},
		}
	}
	w := startTestWindow(t, app, Options{Title: "refresh-commit", Width: 200, Height: 400})
	waitForTitle(t, w, "refresh-commit|0")
	var scroll *view
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v != nil && v.kind == driver.KindScroll {
				scroll = v
			}
		}
	})
	if scroll == nil {
		t.Fatal("no scroll view")
	}
	waitRest := func(want driver.Points) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var at driver.Points
			var moving bool
			w.onLoop(t, func() { at, moving = scroll.offset.Y, scroll.glide != nil })
			if !moving && at == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("offset = %v, moving = %v; want rest at %v", at, moving, want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitRest(-56)
	change := func(step int, update func(*state), want driver.Points) {
		t.Helper()
		root.Post(func() {
			s := value.Get()
			s.step = step
			update(&s)
			value.Set(s)
		})
		waitForTitle(t, w, fmt.Sprintf("refresh-commit|%d", step))
		waitRest(want)
	}
	change(1, func(s *state) { s.bottom = 20 }, -56)
	change(2, func(s *state) { s.content = 1200 }, -56)
	change(3, func(s *state) { s.height = 280 }, -56)
	change(4, func(s *state) { s.top = 10 }, -66)
	change(5, func(s *state) { s.indicator = 80 }, -90)
	change(6, func(s *state) { s.refreshing = false }, -10)
	change(7, func(s *state) { s.refreshing = true }, -90)
	// An explicit command still uses the ordinary content bounds.
	w.onLoop(t, func() {
		w.interruptScroll(scroll)
		w.scrollTo(scroll, driver.Point{})
	})
	waitRest(0)
	change(8, func(s *state) { s.bottom = 30 }, 0)
}
