package core

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
)

// recorder is a storage backend that loads kept and records changes.
type recorder struct {
	kept    map[string]string
	err     error
	mu      sync.Mutex
	changes []StorageChange
}

func (r *recorder) Load() (map[string]string, error) { return r.kept, r.err }

func (r *recorder) Write(c StorageChange) {
	r.mu.Lock()
	r.changes = append(r.changes, c)
	r.mu.Unlock()
}

type checkedRecorder struct {
	recorder
	refuse func(StorageChange) error
}

func (r *checkedRecorder) WriteErr(c StorageChange) error {
	if err := r.refuse(c); err != nil {
		return err
	}
	r.recorder.Write(c)
	return nil
}

// graph is a root whose posts run when drained.
type graph struct {
	root  *Scope
	posts []func()
}

func newGraph(store *Store) *graph {
	g := &graph{}
	g.root = testRoot(RootOptions{Post: func(fn func()) { g.posts = append(g.posts, fn) }, Storage: store})
	return g
}

func (g *graph) drain() {
	for len(g.posts) > 0 {
		p := g.posts
		g.posts = nil
		for _, fn := range p {
			fn()
		}
	}
}

func TestStorageLoadsOnceAndWritesChanges(t *testing.T) {
	b := &recorder{kept: map[string]string{"a": "1"}}
	s := newGraph(NewStore(b)).root.Storage()
	if v, ok := s.GetItem("a"); !ok || v != "1" {
		t.Fatalf("a = %q, %v", v, ok)
	}
	if err := s.SetItem("b", "2"); err != nil {
		t.Fatal(err)
	}
	s.SetItem("b", "2")
	s.RemoveItem("a")
	s.RemoveItem("missing")
	if k := s.Keys(); !slices.Equal(k, []string{"b"}) {
		t.Fatalf("keys = %v", k)
	}
	s.Clear()
	s.Clear()
	want := []StorageChange{{Op: StorageSet, Key: "b", Value: "2"}, {Op: StorageRemove, Key: "a"}, {Op: StorageClear}}
	if !slices.Equal(b.changes, want) {
		t.Fatalf("changes = %v, want %v", b.changes, want)
	}
}

func TestStorageQuota(t *testing.T) {
	b := &recorder{kept: map[string]string{"k": strings.Repeat("v", StorageQuota-1)}}
	s := newGraph(NewStore(b)).root.Storage()
	// What was loaded counts.
	var full *StorageFullError
	if err := s.SetItem("a", "b"); !errors.As(err, &full) || full.Bytes != StorageQuota+2 {
		t.Fatalf("err = %v", err)
	}
	if _, ok := s.GetItem("a"); ok {
		t.Fatal("a refused value was kept")
	}
	// Replacing a value counts the new one only.
	if err := s.SetItem("k", strings.Repeat("z", StorageQuota-1)); err != nil {
		t.Fatal(err)
	}
	s.RemoveItem("k")
	// "键" is three bytes, "😀" four.
	if err := s.SetItem("键", strings.Repeat("😀", (StorageQuota-3)/4)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetItem("x", strings.Repeat("y", 4)); err == nil {
		t.Fatal("the quota counted runes, not bytes")
	}
}

func TestStorageFailedLoadBlocksChanges(t *testing.T) {
	readErr := errors.New("unreadable")
	b := &recorder{kept: map[string]string{"partial": "ignored"}, err: readErr}
	s := newGraph(NewStore(b)).root.Storage()
	if err := s.Err(); !errors.Is(err, readErr) {
		t.Fatalf("Err = %v, want %v", err, readErr)
	}
	b.err = nil // A failed load is not retried for this Store.
	if _, ok := s.GetItem("a"); ok {
		t.Fatal("a has a value")
	}
	if err := s.SetItem("a", "1"); !errors.Is(err, readErr) {
		t.Fatalf("SetItem = %v, want %v", err, readErr)
	}
	s.RemoveItem("partial")
	s.Clear()
	if err := s.Err(); !errors.Is(err, readErr) || len(s.Keys()) != 0 || len(b.changes) != 0 {
		t.Fatalf("failed load: Err = %v, keys = %v, writes = %v", err, s.Keys(), b.changes)
	}
}

func TestStorageFailedWritesKeepEntriesAndWatchers(t *testing.T) {
	writeErr := errors.New("refused")
	refuse := true
	b := &checkedRecorder{
		recorder: recorder{kept: map[string]string{"a": "old"}},
		refuse: func(StorageChange) error {
			if refuse {
				return writeErr
			}
			return nil
		},
	}
	store := NewStore(b)
	a, other := newGraph(store), newGraph(store)
	s := a.root.Storage()
	var reads int
	a.root.Effect(func() { s.GetItem("a"); s.Keys(); reads++ })
	other.root.Storage()

	if err := s.SetItem("a", "new"); !errors.Is(err, writeErr) {
		t.Fatalf("SetItem = %v, want %v", err, writeErr)
	}
	s.RemoveItem("a")
	if err := s.Err(); !errors.Is(err, writeErr) {
		t.Fatalf("RemoveItem's Err = %v", err)
	}
	s.Clear()
	if err := s.Err(); !errors.Is(err, writeErr) {
		t.Fatalf("Clear's Err = %v", err)
	}
	if v, ok := s.GetItem("a"); v != "old" || !ok || reads != 1 || len(other.posts) != 0 || len(b.changes) != 0 {
		t.Fatalf("failed writes: a = %q, %v, reads = %d, posts = %d, writes = %v", v, ok, reads, len(other.posts), b.changes)
	}
	refuse = false
	if err := s.SetItem("a", "new"); err != nil || s.Err() != nil || reads != 2 || len(other.posts) != 1 || len(b.changes) != 1 {
		t.Fatalf("successful retry: SetItem = %v, Err = %v, reads = %d, posts = %d, writes = %v", err, s.Err(), reads, len(other.posts), b.changes)
	}
}

func TestStoragePartitionClearKeepsRefusedKeys(t *testing.T) {
	writeErr := errors.New("refused")
	b := &checkedRecorder{refuse: func(c StorageChange) error {
		if c.Op == StorageRemove && strings.HasSuffix(c.Key, "b") {
			return writeErr
		}
		return nil
	}}
	root := newGraph(NewStore(b)).root
	app := root.Storage()
	part := root.Child()
	part.ProvideStorage("part", 100)
	s := part.Storage()
	for _, key := range []string{"a", "b"} {
		if err := s.SetItem(key, key); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.SetItem("a", "app"); err != nil {
		t.Fatal(err)
	}
	aReads, bReads := 0, 0
	part.Effect(func() { s.GetItem("a"); aReads++ })
	part.Effect(func() { s.GetItem("b"); bReads++ })
	s.Clear()
	if err := s.Err(); !errors.Is(err, writeErr) {
		t.Fatalf("Err = %v", err)
	}
	if keys := s.Keys(); !slices.Equal(keys, []string{"b"}) || aReads != 2 || bReads != 1 {
		t.Fatalf("partition keys = %v, reads = %d, %d", keys, aReads, bReads)
	}
	if v, _ := app.GetItem("a"); v != "app" {
		t.Fatalf("app's key changed to %q", v)
	}
}

func TestStorageCustomQuota(t *testing.T) {
	s := newGraph(NewStoreWithQuota(nil, 10)).root.Storage()
	if err := s.SetItem("k", "123456789"); err != nil {
		t.Fatal(err)
	}
	var full *StorageFullError
	if err := s.SetItem("a", "b"); !errors.As(err, &full) || full.Quota != 10 {
		t.Fatalf("SetItem = %v, want StorageFullError with quota 10", err)
	}
}

func TestStorageSignalsChanges(t *testing.T) {
	store := NewStore(nil)
	a, b := newGraph(store), newGraph(store)
	var seen []string
	var keys int
	a.root.Effect(func() {
		v, _ := a.root.Storage().GetItem("draft")
		seen = append(seen, v)
	})
	a.root.Effect(func() {
		a.root.Storage().Keys()
		keys++
	})
	sa, sb := a.root.Storage(), b.root.Storage()

	// The graph's own change reaches its effects at once.
	sa.SetItem("draft", "one")
	// Another graph's comes through a post.
	sb.SetItem("draft", "two")
	if seen[len(seen)-1] != "one" {
		t.Fatalf("seen = %v before the post ran", seen)
	}
	a.drain()
	// A change to another key leaves the draft's readers alone.
	sb.SetItem("other", "x")
	a.drain()
	sb.Clear()
	a.drain()
	if want := []string{"", "one", "two", ""}; !slices.Equal(seen, want) {
		t.Fatalf("seen = %v, want %v", seen, want)
	}
	if keys != 5 {
		t.Fatalf("keys read %d times, want 5", keys)
	}

	// A disposed graph stops watching.
	a.root.Dispose()
	sb.SetItem("draft", "three")
	if len(a.posts) != 0 {
		t.Fatal("a disposed graph was told of a change")
	}
}

func TestStorageInMemoryByDefault(t *testing.T) {
	a, b := newGraph(nil), newGraph(nil)
	a.root.Storage().SetItem("draft", "saved")
	if _, ok := b.root.Storage().GetItem("draft"); ok {
		t.Fatal("graphs without a store shared entries")
	}
}

func TestScopeStoreCanBeSharedWithAnotherRoot(t *testing.T) {
	first := newGraph(nil)
	store := first.root.Store()
	second := newGraph(store)
	if first.root.Store() != store || second.root.Store() != store {
		t.Fatal("roots did not return their shared store")
	}
	if err := first.root.Storage().SetItem("draft", "saved"); err != nil {
		t.Fatal(err)
	}
	if got, ok := second.root.Storage().GetItem("draft"); !ok || got != "saved" {
		t.Fatalf("second root read %q, %v", got, ok)
	}
}
