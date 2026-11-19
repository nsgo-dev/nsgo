package tsx

import (
	"time"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"
)

// deadline is how long a plugin's work may run while Limits.CallTotal holds it: a call's from Plugin.Call, the entry module's from Load, held to Limits.Load, and that of what the host delivers on its own, such as an event, from its delivery. What the work sets going while it runs, a timer, an interval, a frame, a microtask or a host request's answer, carries the same deadline, however late it runs and through however many of them, and once the deadline passes all of it is dropped, intervals cleared: no work outlives the deadline it started under, and none takes another's. It lives on the engine's goroutine; only its watchdog fires on another, and posts.
type deadline struct {
	at      time.Time
	expired bool
	// timers are the ids of the timers it set, which expire stops.
	timers map[int64]struct{}
	// onExpire fails the call it is the deadline of, nil for others.
	onExpire func()
	// watch is the timer that expires it on time, set once work waits on it.
	watch *time.Timer
}

// newDeadline returns a deadline d from now, watched at once when watched, as a call's is, which must fail when it passes even while nothing runs.
func (r *runtime) newDeadline(d time.Duration, onExpire func(), watched bool) *deadline {
	dl := &deadline{at: time.Now().Add(d), onExpire: onExpire}
	if watched {
		r.watch(dl)
	}
	return dl
}

// watch has dl expire on time from the engine's goroutine, once something waits on it.
func (r *runtime) watch(dl *deadline) {
	if dl.watch != nil || dl.expired {
		return
	}
	dl.watch = time.AfterFunc(time.Until(dl.at), func() { r.cx.Post(func() { r.expire(dl) }) })
}

// passed reports whether dl has run out, expiring it if it just did.
func (r *runtime) passed(dl *deadline) bool {
	if !dl.expired && !time.Now().Before(dl.at) {
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
	}
	for id := range dl.timers {
		if stop := r.timers.stop[id]; stop != nil {
			stop()
			delete(r.timers.stop, id)
		}
	}
	clear(dl.timers)
	if dl.onExpire != nil {
		dl.onExpire()
	}
}

// under runs fn, work dl's started, as dl's: not at all once dl passed. Outside a call into JS, fn's calls into JS run under dl; inside one, dl joins it (adopt).
func (r *runtime) under(dl *deadline, fn func()) {
	if dl == nil {
		fn()
		return
	}
	if r.passed(dl) {
		return
	}
	if r.depth > 0 {
		r.adopt(dl)
		fn()
		return
	}
	prev := r.current
	r.current = dl
	defer func() { r.current = prev }()
	fn()
}

// adopt has the call into JS running hold to dl as well, when dl is the sooner: work two deadlines meet in, as a host request's answer in another's call, runs under the earlier, so it extends neither.
func (r *runtime) adopt(dl *deadline) {
	if r.current != nil && !dl.at.Before(r.current.at) {
		return
	}
	r.current = dl
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
