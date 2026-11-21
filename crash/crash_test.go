package crash

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStoreWriteOrderAndAcknowledge(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "crashes")
	if runtime.GOOS == "js" {
		dir = t.Name()
	}
	store := Open(dir)
	t.Cleanup(func() { _ = store.AcknowledgeAll() })

	t1 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 20, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 20, 11, 0, 0, 0, time.UTC)

	// Write out of chronological order to verify sorting.
	if _, err := store.WriteReport(CrashReport{
		ID:          "b-second-tie",
		Kind:        KindNativeException,
		AppVersion:  "1.1.0",
		BuildNumber: "11",
		OS:          "darwin",
		Arch:        "arm64",
		Timestamp:   t3,
		Trace:       "NSException: NSRangeException",
	}); err != nil {
		t.Fatalf("WriteReport b: %v", err)
	}
	if _, err := store.WriteReport(CrashReport{
		ID:          "a-first",
		Kind:        KindGoFatal,
		AppVersion:  "1.0.0",
		BuildNumber: "10",
		OS:          "linux",
		Arch:        "amd64",
		Timestamp:   t1,
		Trace:       "fatal error: runtime",
	}); err != nil {
		t.Fatalf("WriteReport a: %v", err)
	}
	if _, err := store.WriteReport(CrashReport{
		ID:          "a-second-tie",
		Kind:        KindPanic,
		AppVersion:  "1.1.0",
		BuildNumber: "11",
		OS:          "windows",
		Arch:        "amd64",
		Timestamp:   t2,
		Trace:       "panic: nil pointer",
	}); err != nil {
		t.Fatalf("WriteReport tie: %v", err)
	}

	reports, err := store.PendingReports()
	if err != nil {
		t.Fatalf("PendingReports: %v", err)
	}
	if len(reports) != 3 {
		t.Fatalf("got %d reports, want 3", len(reports))
	}
	wantIDs := []string{"a-first", "a-second-tie", "b-second-tie"}
	for i, want := range wantIDs {
		if reports[i].ID != want {
			t.Errorf("reports[%d].ID = %q, want %q", i, reports[i].ID, want)
		}
	}

	// Acknowledge one report and ensure it never returns.
	if err := store.Acknowledge("a-second-tie"); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	// Acknowledging an already-removed ID is idempotent.
	if err := store.Acknowledge("a-second-tie"); err != nil {
		t.Fatalf("second Acknowledge: %v", err)
	}

	reports, err = store.PendingReports()
	if err != nil {
		t.Fatalf("PendingReports after acknowledge: %v", err)
	}
	if len(reports) != 2 || reports[0].ID != "a-first" || reports[1].ID != "b-second-tie" {
		t.Fatalf("unexpected remaining reports: %+v", reports)
	}

	// Reject unsafe IDs.
	for _, bad := range []string{"", ".", "..", "../escape", "sub/id", `sub\id`, "bad\x00id"} {
		if err := store.Acknowledge(bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Acknowledge(%q) = %v, want ErrInvalidID", bad, err)
		}
		if bad != "" {
			if _, err := store.WriteReport(CrashReport{ID: bad, Trace: "x"}); !errors.Is(err, ErrInvalidID) {
				t.Errorf("WriteReport(%q) = %v, want ErrInvalidID", bad, err)
			}
		}
	}

	// AcknowledgeAll removes every remaining report.
	if err := store.AcknowledgeAll(); err != nil {
		t.Fatalf("AcknowledgeAll: %v", err)
	}
	reports, err = store.PendingReports()
	if err != nil {
		t.Fatalf("PendingReports after AcknowledgeAll: %v", err)
	}
	if len(reports) != 0 {
		t.Fatalf("got %d reports after AcknowledgeAll, want 0", len(reports))
	}
}

func TestRecordPanic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "crashes")
	if runtime.GOOS == "js" {
		dir = t.Name()
	}
	fixed := time.Date(2026, 11, 21, 9, 30, 0, 0, time.UTC)
	store := &Store{
		dir: dir,
		meta: sessionMeta{
			AppVersion:  "2.1.0",
			BuildNumber: "210",
			OS:          runtime.GOOS,
			Arch:        runtime.GOARCH,
		},
		now: func() time.Time { return fixed },
	}
	t.Cleanup(func() { _ = store.AcknowledgeAll() })

	if err := store.RecordPanic("boom", []byte("goroutine 1 [running]:\nmain.go:42")); err != nil {
		t.Fatalf("RecordPanic: %v", err)
	}
	reports, err := store.PendingReports()
	if err != nil {
		t.Fatalf("PendingReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("got %d reports, want 1", len(reports))
	}
	r := reports[0]
	if r.Kind != KindPanic {
		t.Errorf("Kind = %q, want %q", r.Kind, KindPanic)
	}
	if r.AppVersion != "2.1.0" || r.BuildNumber != "210" {
		t.Errorf("version/build = %q/%q, want 2.1.0/210", r.AppVersion, r.BuildNumber)
	}
	if !r.Timestamp.Equal(fixed) {
		t.Errorf("Timestamp = %v, want %v", r.Timestamp, fixed)
	}
	if !strings.Contains(r.Trace, "panic: boom") || !strings.Contains(r.Trace, "main.go:42") {
		t.Errorf("unexpected Trace: %q", r.Trace)
	}
}
