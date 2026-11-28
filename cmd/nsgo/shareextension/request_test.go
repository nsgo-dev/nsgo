//go:build ios

package shareextension

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// clock runs a request's timers when the test says.
type clock struct {
	mu     sync.Mutex
	timers []timer
}

type timer struct {
	d  time.Duration
	fn func()
}

func (c *clock) after(d time.Duration, fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timers = append(c.timers, timer{d, fn})
}

// fire runs the timers set for d, reporting whether there were any.
func (c *clock) fire(d time.Duration) bool {
	c.mu.Lock()
	var due []func()
	rest := c.timers[:0]
	for _, t := range c.timers {
		if t.d == d {
			due = append(due, t.fn)
		} else {
			rest = append(rest, t)
		}
	}
	c.timers = rest
	c.mu.Unlock()
	for _, fn := range due {
		fn()
	}
	return len(due) > 0
}

type outcome struct {
	shown, finished []error
}

func testRequest() (*request, *clock, *outcome) {
	c, o := &clock{}, &outcome{}
	r := newRequest(func(err error) { o.shown = append(o.shown, err) }, func(err error) { o.finished = append(o.finished, err) })
	r.after = c.after
	return r, c, o
}

// The request completes only once its view has appeared and its import has finished, whichever comes last, and after the sheet has said so: an import that finishes first waits for the view.
func TestRequestCompletesAfterAppearAndImport(t *testing.T) {
	for _, importFirst := range []bool{true, false} {
		r, c, o := testRequest()
		if importFirst {
			r.done(nil)
			if c.fire(savedFor) || len(o.finished) > 0 {
				t.Fatal("completed before the view appeared")
			}
			r.appear()
			if c.fire(appearTimeout); len(o.finished) > 0 {
				t.Fatal("the wait for the view closed the request after the view appeared")
			}
		} else {
			r.appear()
			if c.fire(savedFor) || len(o.finished) > 0 {
				t.Fatal("completed before the import finished")
			}
			r.done(nil)
		}
		if len(o.shown) != 1 || o.shown[0] != nil {
			t.Fatalf("shown %v, want the saved confirmation", o.shown)
		}
		if len(o.finished) > 0 {
			t.Fatal("completed without showing the confirmation")
		}
		if !c.fire(savedFor) || len(o.finished) != 1 || o.finished[0] != nil {
			t.Fatalf("import first %v: finished %v, want one completion after savedFor", importFirst, o.finished)
		}
		c.fire(appearTimeout)
		r.appear()
		r.close()
		if len(o.finished) != 1 {
			t.Fatalf("completed %d times", len(o.finished))
		}
	}
}

// A failed import shows its error and cancels the request with it, after failedFor or the user's Done.
func TestRequestFailureCancelsWithTheError(t *testing.T) {
	failure := errors.New("copy failed")
	r, c, o := testRequest()
	r.appear()
	r.done(failure)
	if len(o.shown) != 1 || o.shown[0] != failure {
		t.Fatalf("shown %v, want the failure", o.shown)
	}
	if c.fire(savedFor) || !c.fire(failedFor) || len(o.finished) != 1 || o.finished[0] != failure {
		t.Fatalf("finished %v, want the failure after failedFor", o.finished)
	}

	r, _, o = testRequest()
	r.appear()
	r.done(failure)
	r.close()
	if len(o.finished) != 1 || o.finished[0] != failure {
		t.Fatalf("Done finished %v, want the failure at once", o.finished)
	}
}

// Closing the sheet before the import finishes cancels the request as the user's, stops the import and ignores its late result.
func TestRequestClosedWhileImportingIsCancelled(t *testing.T) {
	r, _, o := testRequest()
	r.appear()
	if r.stopped() {
		t.Fatal("stopped before closing")
	}
	r.close()
	if !r.stopped() || len(o.finished) != 1 || !errors.Is(o.finished[0], errCancelled) {
		t.Fatalf("finished %v, want errCancelled", o.finished)
	}
	r.done(nil)
	if len(o.shown) != 0 || len(o.finished) != 1 {
		t.Fatalf("a late import result was shown (%v) or completed (%v)", o.shown, o.finished)
	}
}

// A view that never appears does not hold the request: it completes appearTimeout after the import.
func TestRequestWithoutAppearCompletes(t *testing.T) {
	r, c, o := testRequest()
	r.done(nil)
	if !c.fire(appearTimeout) || len(o.finished) != 1 || o.finished[0] != nil {
		t.Fatalf("finished %v, want a completion after appearTimeout", o.finished)
	}
}
