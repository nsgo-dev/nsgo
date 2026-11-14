package core

import (
	"sync/atomic"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// KeepAwakeOptions control a power assertion. Display keeps the display on
// as well as preventing idle sleep; false keeps only the computer awake.
type KeepAwakeOptions struct {
	Display bool
}

// KeepAwake keeps the computer from idle sleep until release is called or
// the current scope is disposed. The reason is shown by platforms that list
// power assertions. Releasing more than once is safe.
func (cx *Scope) KeepAwake(reason string, opts KeepAwakeOptions) (release func()) {
	owner := cx.current()
	t := owner.tree()
	if t == nil {
		return func() {}
	}
	t.alertID++
	id := t.alertID
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemKeepAwake, Count: id,
		Awake: &driver.KeepAwake{Reason: reason, Display: opts.Display}})
	t.changed = true
	var done atomic.Bool
	releaseNow := func() {
		if t := owner.tree(); t != nil {
			t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemReleaseAwake, Count: id})
			t.changed = true
		}
	}
	release = func() {
		if !done.CompareAndSwap(false, true) {
			return
		}
		owner.Post(func() {
			releaseNow()
		})
	}
	owner.addCleanup(func() {
		if done.CompareAndSwap(false, true) {
			releaseNow()
		}
	})
	return release
}
