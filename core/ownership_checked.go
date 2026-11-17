//go:build nsgodev || nsgotest || nsgocheck

package core

import goruntime "runtime"

// graphOwner is present only in instrumented builds: a development build records the goroutine of every graph, a test build that of each graph created with RootOptions.CheckCommits.
type graphOwner struct{ goroutine uint64 }

func newGraphOwner(checked bool) graphOwner {
	if developmentOwnership || checked {
		return graphOwner{goroutine: currentGoroutine()}
	}
	return graphOwner{}
}

// check panics when the calling goroutine does not own the graph. Go has no cheap goroutine ID, so the lookup runs only while the graph is idle: inside a batch, a flush or a computation, the call comes from code the graph itself is running on its goroutine, unless another goroutine races with it, which is left to the race detector. Checking every write there would make a development build's commits several times slower.
func (rt *runtime) check() {
	if rt.goroutine == 0 || rt.batch > 0 || rt.flushing || rt.observer != nil {
		return
	}
	if rt.goroutine != currentGoroutine() {
		panic("core: the reactive graph belongs to another goroutine; use Scope.Post")
	}
}

func currentGoroutine() uint64 {
	var header [64]byte
	n := goruntime.Stack(header[:], false)
	var id uint64
	for _, b := range header[len("goroutine "):n] {
		if b < '0' || b > '9' {
			break
		}
		id = id*10 + uint64(b-'0')
	}
	return id
}
