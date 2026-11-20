package tsx

import (
	"time"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"
)

// A deadline owns a plugin call's async work, with or without a total time limit. It also holds the total limits of module loading and host events. Only the engine goroutine uses it; the watchdog posts to that goroutine.
type deadline struct {
	// at is zero when only the budget of each JS slice applies.
	at      time.Time
	expired bool
	// timers are the ids of the timers it set, which expire stops.
	timers map[int64]struct{}
	// done is the pending call's callback, nil once it settles or for work that is not a Plugin.Call.
	done func(any, error)
	// watch is the timer that expires it on time, set once work waits on it.
	watch *time.Timer
}

func newDeadline(d time.Duration) *deadline {
	dl := &deadline{}
	if d > 0 {
		dl.at = time.Now().Add(d)
	}
	return dl
}

// watch has dl expire on time from the engine's goroutine, once something waits on it.
func (r *runtime) watch(dl *deadline) {
	if dl.at.IsZero() || dl.watch != nil || dl.expired {
		return
	}
	dl.watch = time.AfterFunc(time.Until(dl.at), func() { r.cx.Post(func() { r.expire(dl) }) })
}

// passed reports whether dl has run out, expiring it if it just did.
func (r *runtime) passed(dl *deadline) bool {
	if dl == nil {
		return false
	}
	if !dl.expired && !dl.at.IsZero() && !time.Now().Before(dl.at) {
		r.expire(dl)
	}
	return dl.expired
}

// expire drops what dl's work set going and fails its call, once.
func (r *runtime) expire(dl *deadline) {
	if dl.expired {
		return
	}
	dl.expired = true
	if dl.watch != nil {
		dl.watch.Stop()
		dl.watch = nil
	}
	for id := range dl.timers {
		r.stopTimer(id)
	}
	dl.timers = nil
	r.finish(dl, nil, ErrBudget)
}

// under runs work with its owner unless it has already been dropped.
func (r *runtime) under(dl *deadline, fn func()) {
	if dl == nil {
		fn()
		return
	}
	if r.passed(dl) {
		return
	}
	prev := r.current
	r.current = dl
	defer func() { r.current = prev }()
	if r.depth > 0 {
		r.adopt(dl)
	}
	fn()
}

// adopt keeps the earliest total deadline for the JS slice, separately from the owner of the particular callback or promise reaction running.
func (r *runtime) adopt(dl *deadline) {
	if dl.expired || dl.at.IsZero() {
		return
	}
	if earlier := r.sliceDeadline; earlier != nil && !earlier.at.IsZero() && !dl.at.Before(earlier.at) {
		return
	}
	r.sliceDeadline = dl
	if r.slice != nil && dl.at.Before(r.sliceEnd) {
		r.sliceEnd = dl.at
		r.slice.Reset(time.Until(dl.at))
	}
}

// resumer returns a function that runs what the work running now set going, on the engine's goroutine, under the deadline it runs under, dropping it once that passed; the functions run as they are where no deadline holds.
func (r *runtime) resumer() func(fn func()) bool {
	dl := r.current
	if dl == nil {
		return func(fn func()) bool { fn(); return true }
	}
	r.watch(dl)
	return func(fn func()) bool {
		ran := false
		r.under(dl, func() { ran = true; fn() })
		return ran
	}
}

// newPromise is the VM's NewPromise, whose settling, as a host request answers, runs under the deadline of the work that made the promise, and not at all once it passed.
func (r *runtime) newPromise() (*js.Promise, func(any) error, func(any) error) {
	p, resolve, reject := r.vm.NewPromise()
	if r.current == nil {
		return p, resolve, reject
	}
	resume := r.resumer()
	wrap := func(settle func(any) error) func(any) error {
		return func(v any) error {
			var err error
			if r.depth > 0 {
				resume(func() { err = settle(v) })
				return err
			}
			// Settled outside a call into JS, its jobs run in one of their own, under the deadline.
			resume(func() { r.inJS(func() { err = settle(v) }) })
			return err
		}
	}
	return p, wrap(resolve), wrap(reject)
}
