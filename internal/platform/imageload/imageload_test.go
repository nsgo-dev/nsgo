package imageload

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestConcurrentLoadsShareOneFetch(t *testing.T) {
	var hits atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		w.Write([]byte("png-bytes"))
	}))
	defer srv.Close()
	src := core.Src(srv.URL + "/a.png")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if data, err := Load(context.Background(), src, nil); err != nil || string(data) != "png-bytes" {
				t.Errorf("Load = %q, %v", data, err)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if hits.Load() != 1 {
		t.Fatalf("fetched %d times, want once", hits.Load())
	}
	if _, err := Load(context.Background(), src, nil); err != nil || hits.Load() != 1 {
		t.Fatalf("a cached load fetched again (%d) or failed: %v", hits.Load(), err)
	}
}

func TestDataURIAndCancel(t *testing.T) {
	data, err := Load(context.Background(), core.Src("data:image/png;base64,aGk="), nil)
	if err != nil || string(data) != "hi" {
		t.Fatalf("data URI = %q, %v", data, err)
	}
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(block)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := Load(ctx, core.Src(srv.URL+"/slow.png"), nil); err == nil {
		t.Fatal("a cancelled load succeeded")
	}
}

// A load right after the only waiter of a fetch cancelled it, as a canvas cache evicting and drawing a source again does, fetches anew instead of joining the cancelled fetch, and the cancelled fetch, returning later, leaves the new one in flight.
func TestLoadAfterCancelFetchesAnew(t *testing.T) {
	saved := HTTP
	defer func() { HTTP = saved }()
	var hits atomic.Int32
	first, second := make(chan struct{}), make(chan struct{})
	HTTP = func(ctx context.Context, url string, header Header, progress Progress) (*Response, error) {
		switch hits.Add(1) {
		case 1:
			<-first
			return nil, ctx.Err()
		default:
			<-second
			return &Response{Status: 200, Body: []byte("png-bytes")}, nil
		}
	}
	src := core.Src("https://example.invalid/load-after-cancel.png")
	waitFor := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal(what)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancelled := make(chan error, 1)
	go func() {
		_, err := Load(ctx, src, nil)
		cancelled <- err
	}()
	waitFor("the first fetch did not start", func() bool { return hits.Load() == 1 })
	mu.Lock()
	old := inflight[src.URI]
	mu.Unlock()
	cancel()
	if err := <-cancelled; err != context.Canceled {
		t.Fatalf("cancelled Load = %v", err)
	}
	type result struct {
		data []byte
		err  error
	}
	results := make(chan result, 2)
	load := func() {
		data, err := Load(context.Background(), src, nil)
		results <- result{data, err}
	}
	go load()
	waitFor("a load after a cancel joined the cancelled fetch", func() bool { return hits.Load() == 2 })
	close(first)
	<-old.done
	mu.Lock()
	current := inflight[src.URI]
	mu.Unlock()
	if current == nil || current == old {
		t.Fatal("the cancelled fetch's end dropped the new fetch")
	}
	go load()
	time.Sleep(20 * time.Millisecond)
	close(second)
	for range 2 {
		if r := <-results; r.err != nil || string(r.data) != "png-bytes" {
			t.Fatalf("Load = %q, %v", r.data, r.err)
		}
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("fetched %d times, want 2", n)
	}
}

func TestInfoKeepsTheBytesForViews(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("abc"))
	}))
	defer srv.Close()
	it := &driver.MountItem{Kind: driver.ItemImageInfo, Count: 7, Image: &driver.ImageConfig{Source: core.Src(srv.URL + "/b.png")}}
	ev := Info(it, func(data []byte) (driver.Size, error) {
		return driver.Size{Width: driver.Points(len(data)), Height: 1}, nil
	})
	if ev.Kind != driver.EventImageInfo || ev.Count != 7 || ev.Size != (driver.Size{Width: 3, Height: 1}) || ev.Text != "" {
		t.Fatalf("Info = %+v", ev)
	}
	if data, err := Load(context.Background(), it.Image.Source, nil); err != nil || string(data) != "abc" || hits.Load() != 1 {
		t.Fatalf("a view's load after Info = %q, %v, after %d fetches", data, err, hits.Load())
	}
	if ev := Info(&driver.MountItem{Count: 8, Image: &driver.ImageConfig{}}, nil); ev.Count != 8 || ev.Text == "" {
		t.Fatalf("Info of no source = %+v, want an error", ev)
	}
}
