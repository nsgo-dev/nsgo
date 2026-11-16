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

func (o graphOwner) check() {
	if o.goroutine != 0 && o.goroutine != currentGoroutine() {
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
