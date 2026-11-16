package core

import "testing"

func TestAbortDoesNotRunScheduledWork(t *testing.T) {
	cx := NewRoot(RootOptions{})
	ctx := cx.Context()
	signal := NewSignal(cx, 0)
	runs := 0
	cx.Effect(func() { signal.Get(); runs++ })
	cx.Cleanup(func() {
		signal.Set(1)
		cx.Run(func() { t.Error("aborted graph ran app code") })
	})
	cx.Abort()
	if runs != 1 || !cx.Disposed() {
		t.Fatalf("after abort: runs=%d, disposed=%v", runs, cx.Disposed())
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("abort did not cancel the graph's work")
	}
}
