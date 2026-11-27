package processstart

import (
	"sync"
	"sync/atomic"
	"time"
)

var nativeTime = sync.OnceValue(read)
var fixture atomic.Pointer[time.Time]

func Time() time.Time {
	if start := fixture.Load(); start != nil {
		return *start
	}
	return nativeTime()
}

// Pin installs nsgotest's process-wide override. It returns nil if a test
// already owns the fixture, otherwise a release function for test cleanup.
func Pin(start time.Time) func() {
	if !fixture.CompareAndSwap(nil, &start) {
		return nil
	}
	return func() { fixture.Store(nil) }
}
