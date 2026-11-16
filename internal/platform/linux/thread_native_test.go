//go:build linux && !android

package linux

import (
	"syscall"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestNativeWindowsKeepGTKOnItsInitializationThread(t *testing.T) {
	if testing.Short() {
		t.Skip("opens windows")
	}
	var threads []int
	for _, name := range []string{"first", "second", "third"} {
		t.Run(name, func(t *testing.T) {
			w := startTestWindow(t, func(*core.Scope) core.Node { return core.View{} },
				Options{Width: 100, Height: 100})
			w.onLoop(t, func() { threads = append(threads, syscall.Gettid()) })
		})
	}
	if len(threads) == 0 {
		t.Skip("no GTK display")
	}
	for _, thread := range threads {
		if thread != threads[0] {
			t.Fatalf("GTK windows used different OS threads: %v", threads)
		}
	}
}
