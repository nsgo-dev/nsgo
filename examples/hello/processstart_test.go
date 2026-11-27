//go:build !nsgominiprogram && !nsgoharmony

package main

import (
	"runtime"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestProcessStartBeforeApp(t *testing.T) {
	// Sampled before nsgotest.Main can start the platform's test host.
	switch runtime.GOOS {
	case "darwin", "ios", "android", "windows", "linux", "js":
		if testMainProcessStart.IsZero() || !testMainProcessStart.Before(testMainEntered) {
			t.Fatalf("process start %v did not precede TestMain at %v", testMainProcessStart, testMainEntered)
		}
	default:
		if !testMainProcessStart.IsZero() {
			t.Fatalf("unsupported target reported %v", testMainProcessStart)
		}
	}
	if got := core.ProcessStart(); got != testMainProcessStart {
		t.Errorf("process start changed from %v to %v", testMainProcessStart, got)
	}
}
