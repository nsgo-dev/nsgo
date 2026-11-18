package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

type gatedMount struct {
	recorder
	entered chan struct{}
	release chan struct{}
}

func (p *gatedMount) Mount(b *driver.MountBatch) {
	for _, it := range b.Items {
		if it.Kind == driver.ItemText && it.Text.Text == "changed" {
			close(p.entered)
			<-p.release
			break
		}
	}
	p.recorder.Mount(b)
}

func TestSyncWaitsForTheCommitToReachThePlatform(t *testing.T) {
	p := &gatedMount{entered: make(chan struct{}), release: make(chan struct{})}
	var text *core.Signal[string]
	e := Start(p, func(cx *core.Scope) core.Node {
		text = core.NewSignal(cx, "before")
		return core.Text{ContentFunc: text.Get}
	}, Options{Size: driver.Size{Width: 100, Height: 100}})
	t.Cleanup(e.Close)
	var once sync.Once
	release := func() { once.Do(func() { close(p.release) }) }
	t.Cleanup(release)
	e.Sync()
	// Queue the change without waking the engine. Sync's marker wakes that same pass, whose platform mount remains blocked below.
	e.enqueue(message{fn: func() { text.Set("changed") }})
	done := make(chan struct{})
	go func() { e.Sync(); close(done) }()
	select {
	case <-p.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the changed commit did not reach the platform")
	}
	select {
	case <-done:
		t.Fatal("Sync returned before the platform received the commit")
	case <-time.After(100 * time.Millisecond):
	}
	release()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Sync did not return after the commit")
	}
	found := false
	for _, s := range p.texts() {
		found = found || s == "changed"
	}
	if !found {
		t.Fatal("the platform did not receive the changed text before Sync returned")
	}
}

func TestSyncReturnsAfterTheEngineExited(t *testing.T) {
	e := Start(&recorder{}, func(*core.Scope) core.Node { return core.Text{Content: "Hello"} }, Options{})
	e.Close()
	done := make(chan struct{})
	go func() { e.Sync(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Sync waited on an engine that already exited")
	}
}

// TestDispatchKeyDownAnswersWhetherTheViewConsumedTheKey holds the platform's wait for a key down to end with the handler's answer, and with false for a surface the engine does not have and once the engine has exited, so a platform's main thread never waits on nothing (#287).
func TestDispatchKeyDownAnswersWhetherTheViewConsumedTheKey(t *testing.T) {
	p := &recorder{}
	var keys []string
	e := Start(p, func(cx *core.Scope) core.Node {
		return core.View{Focusable: func() bool { return true }, OnKeyDown: func(k core.KeyEvent) bool {
			keys = append(keys, k.Key)
			return k.Key == "k"
		}}
	}, Options{Size: driver.Size{Width: 100, Height: 100}})
	t.Cleanup(e.Close)
	e.Sync()
	var id driver.NodeID
	p.mu.Lock()
	for _, b := range p.batches {
		for _, it := range b.Items {
			if it.Kind == driver.ItemEvents && it.Events.Has(driver.EventKeyDown) {
				id = it.ID
			}
		}
	}
	p.mu.Unlock()
	if id == 0 {
		t.Fatal("no view hears key downs")
	}
	answer := func(e *Engine, key string) bool {
		t.Helper()
		done := make(chan bool, 1)
		go func() { done <- e.DispatchKeyDown(driver.Event{Kind: driver.EventKeyDown, ID: id, Text: key}) }()
		select {
		case consumed := <-done:
			return consumed
		case <-time.After(5 * time.Second):
			t.Fatalf("DispatchKeyDown(%q) did not answer", key)
			return false
		}
	}
	if !answer(e, "k") || answer(e, "j") {
		t.Fatal("the answers are not the handler's")
	}
	if answer(&Engine{runner: e.runner, surface: 99}, "k") {
		t.Fatal("a surface the engine does not have consumed a key")
	}
	e.Close()
	if answer(e, "k") {
		t.Fatal("an exited engine consumed a key")
	}
	if len(keys) != 2 || keys[0] != "k" || keys[1] != "j" {
		t.Fatalf("the handler heard %v, want [k j]", keys)
	}
}
