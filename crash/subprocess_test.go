//go:build !js

package crash

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

const (
	crashHelperEnv    = "NSGO_CRASH_SUBPROCESS_HELPER"
	crashHelperDirEnv = "NSGO_CRASH_SUBPROCESS_DIR"
)

// TestRealSubprocessFatalConcurrentMapWrite launches a real child test binary
// with crash.Install active, triggers a deliberate fatal concurrent map write
// that aborts the Go runtime without recover(), and then verifies on the "next
// launch" that:
//  1. A pending CrashReport is promoted with KindGoFatal and the real Go
//     goroutine stack trace ("fatal error: concurrent map writes").
//  2. AppVersion, BuildNumber, OS and Arch come from the crashing launch's
//     startup metadata sidecar even after the next launch bumps the version.
//  3. Acknowledge(report.ID) removes the report permanently from disk.
func TestRealSubprocessFatalConcurrentMapWrite(t *testing.T) {
	if os.Getenv(crashHelperEnv) == "concurrent_map_write" {
		dir := os.Getenv(crashHelperDirEnv)
		if err := Install(Options{
			Dir:         dir,
			AppVersion:  "3.4.5",
			BuildNumber: "789",
		}); err != nil {
			t.Fatalf("child Install: %v", err)
		}
		// Trigger a real, unrecoverable Go runtime fatal error.
		m := make(map[int]int)
		go func() {
			for i := 0; ; i++ {
				m[i] = i
			}
		}()
		for i := 0; ; i++ {
			m[i] = i
		}
	}

	dir := filepath.Join(t.TempDir(), "crashes")
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=^TestRealSubprocessFatalConcurrentMapWrite$")
	cmd.Env = append(os.Environ(),
		crashHelperEnv+"=concurrent_map_write",
		crashHelperDirEnv+"="+dir,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected child process to crash with fatal error, but it exited 0\noutput:\n%s", out)
	}
	if !strings.Contains(string(out), "fatal error: concurrent map") {
		t.Fatalf("expected child stderr to report concurrent map fatal error, got:\n%s", out)
	}

	// Simulate the next launch with a newer app version and build number.
	t.Cleanup(func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) })
	if err := Install(Options{
		Dir:         dir,
		AppVersion:  "4.0.0",
		BuildNumber: "900",
	}); err != nil {
		t.Fatalf("next-launch Install: %v", err)
	}

	reports, err := PendingReports()
	if err != nil {
		t.Fatalf("PendingReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("got %d pending reports, want 1 (child output:\n%s)", len(reports), out)
	}
	r := reports[0]
	if r.ID == "" {
		t.Error("expected non-empty report ID")
	}
	if r.Kind != KindGoFatal {
		t.Errorf("Kind = %q, want %q", r.Kind, KindGoFatal)
	}
	if r.AppVersion != "3.4.5" {
		t.Errorf("AppVersion = %q, want %q (from crashing launch sidecar)", r.AppVersion, "3.4.5")
	}
	if r.BuildNumber != "789" {
		t.Errorf("BuildNumber = %q, want %q (from crashing launch sidecar)", r.BuildNumber, "789")
	}
	if r.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", r.OS, runtime.GOOS)
	}
	if r.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", r.Arch, runtime.GOARCH)
	}
	if r.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
	if !strings.Contains(r.Trace, "goroutine") {
		t.Errorf("Trace does not contain goroutine dump:\n%s", r.Trace)
	}
	if !strings.Contains(r.Trace, "maps.fatal") && !strings.Contains(r.Trace, "fatal error: concurrent map") {
		t.Errorf("Trace does not contain concurrent map fatal frame:\n%s", r.Trace)
	}
	if !strings.Contains(r.Trace, "TestRealSubprocessFatalConcurrentMapWrite") {
		t.Errorf("Trace does not contain crashing test frame:\n%s", r.Trace)
	}

	// Acknowledge must delete the report permanently from disk.
	if err := Acknowledge(r.ID); err != nil {
		t.Fatalf("Acknowledge(%q): %v", r.ID, err)
	}
	after, err := PendingReports()
	if err != nil {
		t.Fatalf("PendingReports after Acknowledge: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("got %d reports after Acknowledge, want 0", len(after))
	}

	// A clean subsequent launch with an empty active-go.log must produce no reports.
	if err := Install(Options{
		Dir:         dir,
		AppVersion:  "4.0.1",
		BuildNumber: "901",
	}); err != nil {
		t.Fatalf("third-launch Install: %v", err)
	}
	third, err := PendingReports()
	if err != nil {
		t.Fatalf("PendingReports after clean launch: %v", err)
	}
	if len(third) != 0 {
		t.Fatalf("got %d reports after clean launch, want 0", len(third))
	}
}

func TestPromotionPreservesMultipleCrashesAcrossLaunches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "crashes")
	t.Cleanup(func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) })

	// Launch 1 installs and crashes with a Go fatal error.
	if err := Install(Options{
		Dir:         dir,
		AppVersion:  "1.0.0",
		BuildNumber: "10",
		OS:          "linux",
		Arch:        "amd64",
	}); err != nil {
		t.Fatalf("launch 1 Install: %v", err)
	}
	t1 := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	goFile := filepath.Join(dir, activeGoFile)
	if err := os.WriteFile(goFile, []byte("fatal error: out of memory\ngoroutine 1 [running]:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(goFile, t1, t1)

	// Launch 2 promotes launch 1's crash, does NOT acknowledge it, and crashes natively.
	if err := Install(Options{
		Dir:         dir,
		AppVersion:  "2.0.0",
		BuildNumber: "20",
		OS:          "darwin",
		Arch:        "arm64",
	}); err != nil {
		t.Fatalf("launch 2 Install: %v", err)
	}
	t2 := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	nativeFile := filepath.Join(dir, activeNativeFile)
	if err := os.WriteFile(nativeFile, []byte("NSException: NSInvalidArgumentException\nReason: bad selector\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(nativeFile, t2, t2)

	// Launch 3 promotes launch 2's native crash and must see BOTH reports in chronological order.
	if err := Install(Options{
		Dir:         dir,
		AppVersion:  "3.0.0",
		BuildNumber: "30",
	}); err != nil {
		t.Fatalf("launch 3 Install: %v", err)
	}
	reports, err := PendingReports()
	if err != nil {
		t.Fatalf("PendingReports: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want 2: %+v", len(reports), reports)
	}
	if reports[0].Kind != KindGoFatal || reports[0].AppVersion != "1.0.0" || reports[0].BuildNumber != "10" {
		t.Errorf("reports[0] = %+v, want launch 1 go_fatal 1.0.0/10", reports[0])
	}
	if reports[1].Kind != KindNativeException || reports[1].AppVersion != "2.0.0" || reports[1].BuildNumber != "20" {
		t.Errorf("reports[1] = %+v, want launch 2 native_exception 2.0.0/20", reports[1])
	}
}

func TestTraceTruncationOnPromote(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "crashes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	oversized := "fatal error: huge\n" + strings.Repeat("x", maxTraceBytes+4096)
	if err := os.WriteFile(filepath.Join(dir, activeGoFile), []byte(oversized), 0o600); err != nil {
		t.Fatal(err)
	}
	store := Open(dir)
	if err := store.Promote(); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	reports, err := store.PendingReports()
	if err != nil {
		t.Fatalf("PendingReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("got %d reports, want 1", len(reports))
	}
	if !strings.HasSuffix(reports[0].Trace, "... (truncated)\n") {
		t.Errorf("expected truncated suffix, got len=%d tail=%q", len(reports[0].Trace), reports[0].Trace[max(0, len(reports[0].Trace)-32):])
	}
}
