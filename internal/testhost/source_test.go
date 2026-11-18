package testhost

import (
	"runtime"
	"testing"
)

// skipRecorder records whether RequireSource skipped.
type skipRecorder struct{ skipped bool }

func (r *skipRecorder) Helper()     {}
func (r *skipRecorder) Skip(...any) { r.skipped = true }

// TestRequireSourceSkipsOnlyAwayFromTheSource runs a source check on the development machine and skips it once a backend's test host is registered, as it is in a target's test binary.
func TestRequireSourceSkipsOnlyAwayFromTheSource(t *testing.T) {
	if runtime.GOOS == "js" || Current() != nil {
		t.Skip("runs on the development machine without a test host")
	}
	var r skipRecorder
	RequireSource(&r)
	if r.skipped {
		t.Fatal("skipped on the development machine")
	}
	Register(struct{ Host }{})
	t.Cleanup(func() { Register(nil) })
	RequireSource(&r)
	if !r.skipped {
		t.Fatal("ran under a backend's test host")
	}
}
