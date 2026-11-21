// Package crash captures fatal Go errors, panics and native uncaught
// exceptions so the next launch of an NSGo app can inspect, upload and
// acknowledge them.
//
// nsgo.Run calls Install automatically at startup, routing Go's runtime crash
// output (runtime/debug.SetCrashOutput) and each platform's native uncaught
// exception handler to a crashes subdirectory under the app's LogsDir (or
// localStorage / in-memory storage on the web). Any non-empty crash files left
// by a previous run are promoted into pending CrashReport entries at startup
// before the new session opens its own crash output files, preserving the
// crashing run's AppVersion, BuildNumber, OS and Arch from its startup
// metadata sidecar.
//
// Inside an NSGo view or scope, call Pending(cx), AcknowledgeIn(cx, id) and
// AcknowledgeAllIn(cx) so tests running in parallel with nsgotest stay
// isolated to each app's LogsDir. Process-level callers outside a scope may
// call PendingReports, Acknowledge and AcknowledgeAll directly.
package crash

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// Report kinds stored in CrashReport.Kind.
const (
	// KindGoFatal is a fatal Go runtime error or unhandled OS signal written
	// through runtime/debug.SetCrashOutput (for example, concurrent map writes).
	KindGoFatal = "go_fatal"
	// KindNativeException is an uncaught platform exception captured outside
	// Go's runtime (NSException on macOS/iOS, SEH exception on Windows, Java
	// uncaught exception on Android, or window error/unhandledrejection on web).
	KindNativeException = "native_exception"
	// KindPanic is a Go panic, either an unhandled panic written to the crash
	// output file or a recovered panic recorded via RecordPanic.
	KindPanic = "panic"
)

// ErrInvalidID reports an empty or unsafe crash report identifier.
var ErrInvalidID = errors.New("crash: invalid report id")

// maxTraceBytes caps the trace payload read from a raw crash file so a huge
// goroutine dump cannot exhaust memory at launch.
const maxTraceBytes = 1 << 20 // 1 MiB

// maxPendingReports caps the number of retained unacknowledged reports during
// promotion so a crash-looping app cannot fill disk unboundedly.
const maxPendingReports = 100

// CrashReport describes a crash captured from a previous (or current) launch.
type CrashReport struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	AppVersion  string    `json:"app_version"`
	BuildNumber string    `json:"build_number"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	Timestamp   time.Time `json:"timestamp"`
	Trace       string    `json:"trace"`
}

// Options configures Install. Zero fields take sensible defaults from
// core.App(), the runtime platform and the app's logs directory.
type Options struct {
	// Dir is the directory holding active crash files, metadata sidecars and
	// pending reports. When empty on native targets, Install resolves the
	// app's logs directory from core.App().ID. On the web, Dir is used as an
	// optional storage namespace key.
	Dir string
	// AppVersion and BuildNumber override core.App().Version and
	// core.App().Build when non-empty.
	AppVersion  string
	BuildNumber string
	// OS and Arch override runtime.GOOS and runtime.GOARCH when non-empty.
	OS   string
	Arch string
	// Now provides timestamps; time.Now is used when nil.
	Now func() time.Time
}

// sessionMeta is written at startup as a sidecar alongside the active crash
// output files so an abrupt fatal crash preserves the crashing build's
// metadata even if the user upgrades before the next launch.
type sessionMeta struct {
	AppVersion  string    `json:"app_version"`
	BuildNumber string    `json:"build_number"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	StartedAt   time.Time `json:"started_at"`
}

var (
	defaultMu    sync.RWMutex
	defaultStore *Store
	idSeq        atomic.Uint64
)

// Store reads, writes and acknowledges pending crash reports in one directory
// (or web storage namespace).
type Store struct {
	dir  string
	meta sessionMeta
	now  func() time.Time
}

// Open returns a Store backed by dir (on native targets) or namespace dir (on
// the web). Open does not modify global crash output handlers.
func Open(dir string) *Store {
	app := core.App()
	return &Store{
		dir: dir,
		meta: sessionMeta{
			AppVersion:  app.Version,
			BuildNumber: app.Build,
			OS:          runtime.GOOS,
			Arch:        runtime.GOARCH,
		},
		now: time.Now,
	}
}

// DirForLogs returns the crashes directory under an app's LogsDir, or "" if
// logsDir is empty (as on the web).
func DirForLogs(logsDir string) string {
	if logsDir == "" {
		return ""
	}
	return filepath.Join(logsDir, "crashes")
}

// Install rotates any non-empty crash files from a previous run into pending
// reports, records the current session's metadata sidecar, routes Go's
// runtime/debug.SetCrashOutput to a fresh crash file, and installs the
// platform's native uncaught exception handler.
func Install(opts Options) error {
	return installPlatform(opts)
}

// PendingReports returns all unacknowledged crash reports from the installed
// process store in chronological order (oldest first).
func PendingReports() ([]CrashReport, error) {
	s := currentDefaultStore()
	if s == nil {
		return nil, nil
	}
	return s.PendingReports()
}

// Acknowledge atomically deletes the pending crash report with id from the
// installed process store so it will not be returned again.
func Acknowledge(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	s := currentDefaultStore()
	if s == nil {
		return nil
	}
	return s.Acknowledge(id)
}

// AcknowledgeAll deletes all pending crash reports from the installed process
// store.
func AcknowledgeAll() error {
	s := currentDefaultStore()
	if s == nil {
		return nil
	}
	return s.AcknowledgeAll()
}

// RecordPanic persists a recovered Go panic as a pending KindPanic report in
// the installed process store.
func RecordPanic(value any, stack []byte) error {
	s := currentDefaultStore()
	if s == nil {
		return nil
	}
	return s.RecordPanic(value, stack)
}

// Pending returns all unacknowledged crash reports for cx's app in
// chronological order (oldest first). When cx has a LogsDir, it reads from
// DirForLogs(cx.LogsDir()); otherwise (such as on the web) it uses the
// installed default store.
func Pending(cx *core.Scope) ([]CrashReport, error) {
	s := storeForScope(cx)
	if s == nil {
		return nil, nil
	}
	return s.PendingReports()
}

// AcknowledgeIn atomically deletes the pending crash report with id from cx's
// app crash store.
func AcknowledgeIn(cx *core.Scope, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	s := storeForScope(cx)
	if s == nil {
		return nil
	}
	return s.Acknowledge(id)
}

// AcknowledgeAllIn deletes all pending crash reports from cx's app crash store.
func AcknowledgeAllIn(cx *core.Scope) error {
	s := storeForScope(cx)
	if s == nil {
		return nil
	}
	return s.AcknowledgeAll()
}

// RecordPanic persists a recovered Go panic into s as a KindPanic report.
func (s *Store) RecordPanic(value any, stack []byte) error {
	trace := fmt.Sprintf("panic: %v", value)
	if len(stack) > 0 {
		trace += "\n\n" + string(stack)
	}
	if len(trace) > maxTraceBytes {
		trace = trace[:maxTraceBytes] + "\n... (truncated)\n"
	}
	_, err := s.WriteReport(CrashReport{
		Kind:        KindPanic,
		AppVersion:  s.meta.AppVersion,
		BuildNumber: s.meta.BuildNumber,
		OS:          s.meta.OS,
		Arch:        s.meta.Arch,
		Timestamp:   s.nowTime(),
		Trace:       trace,
	})
	return err
}

func storeForScope(cx *core.Scope) *Store {
	if cx != nil {
		if logs := cx.LogsDir(); logs != "" {
			return Open(DirForLogs(logs))
		}
	}
	return currentDefaultStore()
}

func currentDefaultStore() *Store {
	defaultMu.RLock()
	s := defaultStore
	defaultMu.RUnlock()
	return s
}

func setDefaultStore(s *Store) {
	defaultMu.Lock()
	defaultStore = s
	defaultMu.Unlock()
}

func (s *Store) nowTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func resolveSessionMeta(opts Options, now time.Time) sessionMeta {
	app := core.App()
	m := sessionMeta{
		AppVersion:  cmp.Or(opts.AppVersion, app.Version),
		BuildNumber: cmp.Or(opts.BuildNumber, app.Build),
		OS:          cmp.Or(opts.OS, runtime.GOOS),
		Arch:        cmp.Or(opts.Arch, runtime.GOARCH),
		StartedAt:   now.UTC(),
	}
	return m
}

func normalizeReport(r CrashReport, meta sessionMeta, now time.Time) (CrashReport, error) {
	if r.Timestamp.IsZero() {
		r.Timestamp = now.UTC()
	} else {
		r.Timestamp = r.Timestamp.UTC()
	}
	if r.ID == "" {
		r.ID = newReportID(r.Timestamp)
	}
	if err := validateID(r.ID); err != nil {
		return CrashReport{}, err
	}
	if r.Kind == "" {
		r.Kind = KindGoFatal
	}
	if r.AppVersion == "" {
		r.AppVersion = meta.AppVersion
	}
	if r.BuildNumber == "" {
		r.BuildNumber = meta.BuildNumber
	}
	if r.OS == "" {
		r.OS = cmp.Or(meta.OS, runtime.GOOS)
	}
	if r.Arch == "" {
		r.Arch = cmp.Or(meta.Arch, runtime.GOARCH)
	}
	return r, nil
}

func newReportID(ts time.Time) string {
	seq := idSeq.Add(1)
	return fmt.Sprintf("%d-%d", ts.UTC().UnixNano(), seq)
}

func validateID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) || strings.ContainsRune(id, 0) {
		return fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	return nil
}

func sortReports(reports []CrashReport) {
	slices.SortFunc(reports, func(a, b CrashReport) int {
		if c := a.Timestamp.Compare(b.Timestamp); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func classifyGoCrash(trace string) string {
	trimmed := strings.TrimSpace(trace)
	if strings.HasPrefix(trimmed, "panic:") {
		return KindPanic
	}
	return KindGoFatal
}
