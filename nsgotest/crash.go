package nsgotest

import (
	"github.com/nsgo-dev/nsgo/crash"
)

// CrashReports starts the app with synthetic pending crash reports in its crash
// store, as if kept from an earlier crashed run, so tests can exercise crash
// upload dialogs and acknowledgement flows. Reports with empty Timestamp take
// the app clock's wall time; empty ID, Kind, OS and Arch receive sensible
// defaults.
func CrashReports(reports ...crash.CrashReport) Option {
	return func(a *App) {
		a.seedCrashReports = append(a.seedCrashReports, reports...)
	}
}

// seedCrashes prepares the app's crash store under logs and writes any
// synthetic reports supplied through CrashReports before the root mounts.
func (a *App) seedCrashes(logs string) {
	a.t.Helper()
	dir := crash.DirForLogs(logs)
	a.crashStore = crash.Open(dir)
	if dir == "" && len(a.seedCrashReports) > 0 {
		_ = crash.Install(crash.Options{})
	}
	var seededIDs []string
	for _, r := range a.seedCrashReports {
		written := a.AddCrashReport(r)
		seededIDs = append(seededIDs, written.ID)
	}
	if dir == "" && len(seededIDs) > 0 {
		a.t.Cleanup(func() {
			for _, id := range seededIDs {
				_ = a.crashStore.Acknowledge(id)
			}
		})
	}
}

// AddCrashReport writes a synthetic pending crash report into the running test
// app's crash store and returns the normalized report (including its assigned
// ID and timestamp).
func (a *App) AddCrashReport(report crash.CrashReport) crash.CrashReport {
	a.t.Helper()
	if a.crashStore == nil {
		a.crashStore = crash.Open("")
	}
	if report.Timestamp.IsZero() && a.Clock != nil {
		report.Timestamp = a.Clock.WallTime()
	}
	written, err := a.crashStore.WriteReport(report)
	if err != nil {
		a.t.Fatalf("nsgotest: write crash report: %v", err)
	}
	return written
}

// PendingCrashReports returns the unacknowledged crash reports currently in the
// test app's crash store, in chronological order.
func (a *App) PendingCrashReports() []crash.CrashReport {
	a.t.Helper()
	if a.crashStore == nil {
		return nil
	}
	reports, err := a.crashStore.PendingReports()
	if err != nil {
		a.t.Fatalf("nsgotest: read pending crash reports: %v", err)
	}
	return reports
}
