//go:build js && wasm && !nsgominiprogram && !nsgoharmony

package main

import (
	"math"
	"syscall/js"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestProcessStartBrowserOrigin(t *testing.T) {
	performance := js.Global().Get("performance")
	const property = "timeOrigin"
	want := performance.Get(property).Float()
	got := float64(testMainProcessStart.UnixMilli()) + float64(testMainProcessStart.Nanosecond()%1e6)/1e6
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("ProcessStart = %f ms, browser reports %f ms", got, want)
	}

	// Change only the test realm's source after the first read to detect
	// implementations that re-read instead of preserving the launch time.
	object := js.Global().Get("Object")
	old := object.Call("getOwnPropertyDescriptor", performance, property)
	t.Cleanup(func() {
		if old.IsUndefined() {
			performance.Delete(property)
		} else {
			object.Call("defineProperty", performance, property, old)
		}
	})
	object.Call("defineProperty", performance, property, map[string]any{"value": want + 1000, "configurable": true})
	if got := core.ProcessStart(); got != testMainProcessStart {
		t.Errorf("ProcessStart changed from %v to %v on a later read", testMainProcessStart, got)
	}
}
