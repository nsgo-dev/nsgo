package nsgotest

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

type settlingSurface struct {
	noSurface
	sync      func()
	animating func() bool
}

// Native edits precede the app's controlled value; applying the two in separate groups used to put the old native text back over that value.
func TestTargetModelKeepsEventsAndCommitsInOrder(t *testing.T) {
	a := Start(t, func(*core.Scope) core.Node { return core.Input{TestID: "field"} })
	v := a.Find("id:field")
	tg := &target{a: a}
	tg.event(driver.Event{Kind: driver.EventChangeText, ID: v.ID, Text: "loud", Count: 1})
	tg.observe(&driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemInputValue, ID: v.ID, Value: "LOUD", Count: 1}}})
	tg.drain()
	if v.Text != "LOUD" {
		t.Fatalf("controlled text = %q, want LOUD", v.Text)
	}
	// A subsequent native edit wins over a preceding commit.
	tg.observe(&driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemInputValue, ID: v.ID, Value: "OTHER", Count: 1}}})
	tg.event(driver.Event{Kind: driver.EventChangeText, ID: v.ID, Text: "OTHER!", Count: 2})
	tg.drain()
	if v.Text != "OTHER!" {
		t.Fatalf("subsequent edit = %q, want OTHER!", v.Text)
	}
	// A slow response to an earlier edit must be rejected just as the native field rejects an ItemInputValue with an old edit count.
	tg.event(driver.Event{Kind: driver.EventChangeText, ID: v.ID, Text: "OTHER!!", Count: 3})
	tg.observe(&driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemInputValue, ID: v.ID, Value: "STALE", Count: 2}}})
	tg.drain()
	if v.Text != "OTHER!!" {
		t.Fatalf("a stale controlled value replaced the latest edit with %q", v.Text)
	}
	// An old view's focus must not reach a new view reusing its ID.
	tg.event(driver.Event{Kind: driver.EventFocus, ID: v.ID})
	tg.observe(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemDelete, ID: v.ID},
		{Kind: driver.ItemCreate, ID: v.ID, View: driver.KindInput},
	}})
	tg.drain()
	if a.views[v.ID].Focused {
		t.Fatal("a focus event for the deleted field reached its replacement")
	}
}

func (s settlingSurface) Sync() { s.sync() }
func (s settlingSurface) Animating() bool {
	return s.animating != nil && s.animating()
}

func settleFixture(t *testing.T) (*target, *View, *driver.MountBatch) {
	t.Helper()
	a := Start(t, func(*core.Scope) core.Node { return core.Text{Content: "before"} })
	v := a.Find("text:before")
	batch := &driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemText, ID: v.ID,
		Text: &driver.TextContent{Text: "after"}}}}
	return &target{a: a}, v, batch
}

func TestTargetSettleWaitsForThePendingCommit(t *testing.T) {
	tg, view, batch := settleFixture(t)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	var calls int
	tg.surface = settlingSurface{sync: func() {
		calls++
		if calls == 1 {
			close(entered)
			<-release
			tg.observe(batch)
		}
	}}
	go func() { tg.settle(); close(done) }()
	t.Cleanup(func() { once.Do(func() { close(release) }); <-done })
	select {
	case <-entered:
	case <-done:
		t.Fatal("Settle returned without waiting for the engine")
	case <-time.After(5 * time.Second):
		t.Fatal("Settle did not synchronize with the engine")
	}
	select {
	case <-done:
		t.Fatal("Settle treated the pending commit as a quiet app")
	case <-time.After(2 * quietFor):
	}
	once.Do(func() { close(release) })
	<-done
	if !tg.quiet || view.Text != "after" {
		t.Fatalf("quiet=%v, text=%q, want the completed commit", tg.quiet, view.Text)
	}
}

func TestTargetSettleChecksTheEngineBeforeReturningQuiet(t *testing.T) {
	tg, view, batch := settleFixture(t)
	calls := 0
	tg.surface = settlingSurface{sync: func() {
		calls++
		if calls == 2 {
			// A commit started after the first barrier but did not send its batch until the second barrier completed.
			tg.observe(batch)
		}
	}}
	tg.settle()
	if !tg.quiet || view.Text != "after" {
		t.Fatalf("quiet=%v, text=%q, want the commit that finished during the quiet interval", tg.quiet, view.Text)
	}
}

func TestTargetSettleBoundsABlockedEngineAndReusesItsWaiter(t *testing.T) {
	tg, _, _ := settleFixture(t)
	release := make(chan struct{})
	var calls atomic.Int32
	tg.surface = settlingSurface{sync: func() { calls.Add(1); <-release }}
	t.Cleanup(func() { close(release); <-tg.syncing })
	for range 3 {
		start := time.Now()
		tg.settleUntil(start.Add(20 * time.Millisecond))
		if tg.quiet || time.Since(start) > time.Second {
			t.Fatalf("quiet=%v after %s with a blocked engine", tg.quiet, time.Since(start))
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("started %d engine waiters, want one across timed-out steps", got)
	}
}

func TestTargetSettleBoundsChangingAppsAndPreservesMeasure(t *testing.T) {
	for _, measure := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "measure"}[measure], func(t *testing.T) {
			tg, _, batch := settleFixture(t)
			tg.a.measure = measure
			tg.surface = settlingSurface{sync: func() {}, animating: func() bool {
				tg.observe(batch) // The app keeps committing between polls.
				return false
			}}
			start := time.Now()
			tg.settleUntil(start.Add(500 * time.Millisecond))
			if tg.quiet != measure {
				t.Fatalf("quiet=%v, want %v for an app that keeps committing", tg.quiet, measure)
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("Settle outlasted its deadline")
			}
		})
	}
}
