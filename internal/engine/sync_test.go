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
