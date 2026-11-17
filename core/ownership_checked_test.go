//go:build (nsgodev || nsgotest || nsgocheck) && !(js && wasm)

package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestGraphOwnership(t *testing.T) {
	posts := make(chan func(), 1)
	cx := NewRoot(RootOptions{CheckCommits: true, Post: func(fn func()) { posts <- fn }})
	t.Cleanup(cx.Dispose)
	signal := NewSignal(cx, 0)
	memo := NewMemo(cx, signal.Get)
	tree := NewTree(cx, ApproxTextEngine{})
	tree.Mount(func(*Scope) Node { return Text{Content: "one"} })
	tree.Commit()
	first, last := cx.first, cx.last
	for name, call := range map[string]func(){
		"set":           func() { signal.Set(1) },
		"equal set":     func() { signal.Set(0) },
		"update":        func() { signal.Update(func(v int) int { return v + 1 }) },
		"effect":        func() { cx.Effect(func() {}) },
		"new memo":      func() { NewMemo(cx, func() int { return 1 }) },
		"run":           func() { cx.Run(func() {}) },
		"batch":         func() { cx.Batch(func() {}) },
		"dispose":       cx.Dispose,
		"quiet commit":  func() { tree.Commit() },
		"shared commit": func() { tree.CommitShared() },
	} {
		t.Run(name, func(t *testing.T) {
			result := make(chan any, 1)
			go func() {
				defer func() { result <- recover() }()
				call()
			}()
			if r := <-result; r == nil || !strings.Contains(fmt.Sprint(r), "Scope.Post") {
				t.Fatalf("wrong-goroutine access panicked with %v, want Scope.Post diagnostic", r)
			}
			if cx.first != first || cx.last != last {
				t.Fatal("rejected access changed the graph's owned scopes")
			}
		})
	}
	posted := make(chan struct{})
	go func() { cx.Post(func() { signal.Set(2) }); close(posted) }()
	<-posted
	(<-posts)()
	if got := memo.Get(); got != 2 {
		t.Fatalf("posted Set: memo = %d, want 2", got)
	}
}

func TestGraphOwnershipFollowsTheRoot(t *testing.T) {
	if developmentOwnership {
		t.Skip("a development build checks every graph")
	}
	unchecked := NewRoot(RootOptions{})
	t.Cleanup(unchecked.Dispose)
	signal := NewSignal(unchecked, 0)
	result := make(chan any, 1)
	go func() {
		defer func() { result <- recover() }()
		signal.Set(1)
	}()
	if r := <-result; r != nil {
		t.Fatalf("a graph created without CheckCommits checked its owner: %v", r)
	}
}
