//go:build darwin && !ios

package crash

import (
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

func TestDarwinNSExceptionCapture(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "crashes")
	t.Cleanup(func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) })

	if err := Install(Options{
		Dir:         dir,
		AppVersion:  "1.5.0",
		BuildNumber: "55",
	}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	makeNSString := func(s string) objc.ID {
		b := append([]byte(s), 0)
		str := objc.ID(objc.GetClass("NSString")).Send(
			objc.RegisterName("stringWithUTF8String:"),
			uintptr(unsafe.Pointer(&b[0])),
		)
		runtime.KeepAlive(b)
		return str
	}

	exc := objc.ID(objc.GetClass("NSException")).Send(
		objc.RegisterName("exceptionWithName:reason:userInfo:"),
		uintptr(makeNSString("NSGoTestException")),
		uintptr(makeNSString("simulated native exception")),
		0,
	)
	if exc == 0 {
		t.Fatal("failed to construct NSException")
	}

	writeDarwinException(exc)

	// Next launch promotes active-native.log using the previous session's metadata.
	if err := Install(Options{
		Dir:         dir,
		AppVersion:  "1.6.0",
		BuildNumber: "60",
	}); err != nil {
		t.Fatalf("second Install: %v", err)
	}

	reports, err := PendingReports()
	if err != nil {
		t.Fatalf("PendingReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("got %d reports, want 1", len(reports))
	}
	r := reports[0]
	if r.Kind != KindNativeException {
		t.Errorf("Kind = %q, want %q", r.Kind, KindNativeException)
	}
	if r.AppVersion != "1.5.0" || r.BuildNumber != "55" {
		t.Errorf("version/build = %q/%q, want 1.5.0/55", r.AppVersion, r.BuildNumber)
	}
	if !strings.Contains(r.Trace, "NSException: NSGoTestException") {
		t.Errorf("Trace missing exception name:\n%s", r.Trace)
	}
	if !strings.Contains(r.Trace, "Reason: simulated native exception") {
		t.Errorf("Trace missing exception reason:\n%s", r.Trace)
	}
}
