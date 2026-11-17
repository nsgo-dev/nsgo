package press_test

import (
	"strconv"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// A completion posted by another goroutine has the same graph ownership
// and ordering headless and on a target, including posts made by a post.
func TestPostsRunOnTheAppGraph(t *testing.T) {
	var value *core.Signal[int]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		value = core.NewSignal(cx, 0)
		return core.Text{ContentFunc: func() string { return strconv.Itoa(value.Get()) }}
	})
	app.Post(func() {
		value.Update(func(n int) int { return n + 1 })
		app.Post(func() { value.Update(func(n int) int { return n + 10 }) })
	})
	queued := make(chan struct{})
	go func() {
		app.Post(func() { value.Update(func(n int) int { return n * 2 }) })
		close(queued)
	}()
	<-queued
	app.Settle()
	app.Find("text:12")
}
