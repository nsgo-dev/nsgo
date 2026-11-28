//go:build ios

package shareextension

import (
	"errors"
	"sync"
	"time"
)

// The request's timings, variables for the tests.
var (
	// savedFor is how long the sheet says the share was saved before it closes on its own.
	savedFor = 1500 * time.Millisecond
	// failedFor is how long it shows a failure before it closes on its own.
	failedFor = 4 * time.Second
	// loadTimeout bounds the wait for each item provider's answer: a provider that never answers would otherwise keep the sheet open on its spinner.
	loadTimeout = 30 * time.Second
	// appearTimeout bounds the wait for the view to appear once the import has finished, for a host that never finishes presenting it.
	appearTimeout = 10 * time.Second
)

// errCancelled is how a request the user closed before its import finished ends.
var errCancelled = errors.New("the user closed the share sheet")

// errTimedOut is how an import whose provider never answered fails.
var errTimedOut = errors.New("the item provider did not answer")

// request is one share request, from viewDidLoad to its completion. The host dismisses the whole share sheet only for a request that completes once the extension's view has finished appearing: one completed earlier, as an import of a link finishes within a few hundred milliseconds, dismisses the extension's view alone and leaves the share sheet open, whose close button then brings back an extension view nothing can dismiss until the host is killed (#2668). So the request completes once the view has appeared and the import has finished, whichever is later, after the sheet has shown the result for savedFor or failedFor; the user's Done or Cancel, or the view disappearing, completes it at once.
type request struct {
	mu                         sync.Mutex
	appeared, imported, closed bool
	err                        error
	// show shows the import's result; finish completes the request, or cancels it with a non-nil error; after runs fn after d. Each may run fn on another goroutine.
	show   func(error)
	finish func(error)
	after  func(d time.Duration, fn func())
}

func newRequest(show, finish func(error)) *request {
	return &request{show: show, finish: finish, after: func(d time.Duration, fn func()) { time.AfterFunc(d, fn) }}
}

// appear records that the view has finished appearing (viewDidAppear:).
func (r *request) appear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.appeared {
		return
	}
	r.appeared = true
	r.schedule()
}

// done records the import's result, err nil when the items wait in the inbox. Only the first result counts.
func (r *request) done(err error) {
	r.mu.Lock()
	if r.imported || r.closed {
		r.mu.Unlock()
		return
	}
	r.imported, r.err = true, err
	r.schedule()
	r.mu.Unlock()
	r.show(err)
}

// stopped reports whether the request has ended, when an import still running should stop and save nothing.
func (r *request) stopped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// schedule closes the request once both the view has appeared and the import has finished, after showing the result; an import that finishes before the view appears waits for it at most appearTimeout.
func (r *request) schedule() {
	switch {
	case r.closed || !r.imported:
	case !r.appeared:
		r.after(appearTimeout, r.closeUnseen)
	case r.err == nil:
		r.after(savedFor, r.close)
	default:
		r.after(failedFor, r.close)
	}
}

// closeUnseen closes a request whose view has still not appeared; once it has, the confirmation's own timer closes it.
func (r *request) closeUnseen() {
	r.mu.Lock()
	appeared := r.appeared
	r.mu.Unlock()
	if !appeared {
		r.close()
	}
}

// close completes the request now: with the import's result once it has finished, else cancelled. Only the first call does.
func (r *request) close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	err := r.err
	if !r.imported {
		err = errCancelled
	}
	r.mu.Unlock()
	r.finish(err)
}
