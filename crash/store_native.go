//go:build !js

package crash

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

const (
	activeMetaFile   = "active.meta.json"
	activeGoFile     = "active-go.log"
	activeNativeFile = "active-native.log"
	reportSuffix     = ".report.json"
)

var installMu sync.Mutex

func installPlatform(opts Options) error {
	installMu.Lock()
	defer installMu.Unlock()

	dir := opts.Dir
	if dir == "" {
		dir = defaultCrashDir()
	}
	if dir == "" {
		return errors.New("crash: no crash directory configured")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crash: create directory: %w", err)
	}

	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()

	// Release any previous duplicated crash-output fd before rotating or
	// truncating active-go.log (required on Windows to avoid sharing violations).
	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})

	if err := promoteActiveLocked(dir, now); err != nil {
		return err
	}

	meta := resolveSessionMeta(opts, now)
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("crash: encode metadata: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(dir, activeMetaFile), metaBytes); err != nil {
		return fmt.Errorf("crash: write metadata sidecar: %w", err)
	}

	goCrashPath := filepath.Join(dir, activeGoFile)
	f, err := os.OpenFile(goCrashPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("crash: open crash output file: %w", err)
	}
	setErr := debug.SetCrashOutput(f, debug.CrashOptions{})
	// SetCrashOutput duplicates the file descriptor, so closing f is safe and
	// prevents leaking the Go file handle.
	closeErr := f.Close()
	if setErr != nil {
		return fmt.Errorf("crash: set crash output: %w", setErr)
	}
	if closeErr != nil {
		return fmt.Errorf("crash: close crash output file: %w", closeErr)
	}

	nativeCrashPath := filepath.Join(dir, activeNativeFile)
	installNativeHandler(nativeCrashPath)

	setDefaultStore(&Store{
		dir:  dir,
		meta: meta,
		now:  nowFn,
	})
	return nil
}

// Promote promotes any non-empty previous-run crash files in s's directory into
// pending report files without modifying global process crash handlers.
func (s *Store) Promote() error {
	if s == nil || s.dir == "" {
		return nil
	}
	return promoteActiveLocked(s.dir, s.nowTime())
}

// promoteActiveLocked inspects active-go.log and active-native.log in dir,
// promotes any non-empty ones using active.meta.json into atomic pending
// report files, and removes the consumed active files.
func promoteActiveLocked(dir string, now time.Time) error {
	meta := readSessionMeta(filepath.Join(dir, activeMetaFile))
	promoted := false

	if ok, err := promoteOne(dir, activeGoFile, "", meta, now); err != nil {
		return err
	} else if ok {
		promoted = true
	}

	if ok, err := promoteOne(dir, activeNativeFile, KindNativeException, meta, now); err != nil {
		return err
	} else if ok {
		promoted = true
	}

	if promoted {
		_ = pruneReports(dir, maxPendingReports)
	}
	return nil
}

func promoteOne(dir, filename, forcedKind string, meta sessionMeta, now time.Time) (bool, error) {
	path := filepath.Join(dir, filename)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("crash: stat %s: %w", filename, err)
	}
	if info.Size() == 0 {
		_ = os.Remove(path)
		return false, nil
	}

	trace, err := readLimitedTrace(path, maxTraceBytes)
	if err != nil {
		return false, fmt.Errorf("crash: read %s: %w", filename, err)
	}
	if len(bytes.TrimSpace([]byte(trace))) == 0 {
		_ = os.Remove(path)
		return false, nil
	}

	ts := info.ModTime().UTC()
	if ts.IsZero() {
		ts = meta.StartedAt.UTC()
	}
	if ts.IsZero() {
		ts = now.UTC()
	}

	kind := forcedKind
	if kind == "" {
		kind = classifyGoCrash(trace)
	}

	report := CrashReport{
		ID:          newReportID(ts),
		Kind:        kind,
		AppVersion:  meta.AppVersion,
		BuildNumber: meta.BuildNumber,
		OS:          meta.OS,
		Arch:        meta.Arch,
		Timestamp:   ts,
		Trace:       trace,
	}
	if report.OS == "" {
		report.OS = runtime.GOOS
	}
	if report.Arch == "" {
		report.Arch = runtime.GOARCH
	}

	data, err := json.Marshal(report)
	if err != nil {
		return false, fmt.Errorf("crash: encode promoted report: %w", err)
	}
	reportPath := filepath.Join(dir, report.ID+reportSuffix)
	if err := writeFileAtomic(reportPath, data); err != nil {
		return false, fmt.Errorf("crash: persist promoted report: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return true, fmt.Errorf("crash: remove promoted %s: %w", filename, err)
	}
	return true, nil
}

// PendingReports returns all unacknowledged crash reports in s sorted in
// chronological order (oldest first).
func (s *Store) PendingReports() ([]CrashReport, error) {
	if s == nil || s.dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("crash: read directory: %w", err)
	}
	var reports []CrashReport
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, reportSuffix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			continue
		}
		var r CrashReport
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		if r.ID == "" {
			r.ID = strings.TrimSuffix(name, reportSuffix)
		}
		reports = append(reports, r)
	}
	sortReports(reports)
	return reports, nil
}

// WriteReport persists r atomically into s as a pending crash report, filling
// any empty ID, Kind, OS, Arch or Timestamp with defaults.
func (s *Store) WriteReport(r CrashReport) (CrashReport, error) {
	if s == nil || s.dir == "" {
		return CrashReport{}, errors.New("crash: store has no directory")
	}
	norm, err := normalizeReport(r, s.meta, s.nowTime())
	if err != nil {
		return CrashReport{}, err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return CrashReport{}, fmt.Errorf("crash: create directory: %w", err)
	}
	data, err := json.Marshal(norm)
	if err != nil {
		return CrashReport{}, fmt.Errorf("crash: encode report: %w", err)
	}
	path := filepath.Join(s.dir, norm.ID+reportSuffix)
	if err := writeFileAtomic(path, data); err != nil {
		return CrashReport{}, fmt.Errorf("crash: write report: %w", err)
	}
	return norm, nil
}

// Acknowledge atomically deletes the pending crash report with id from s.
// Acknowledging an ID that has already been removed succeeds.
func (s *Store) Acknowledge(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if s == nil || s.dir == "" {
		return nil
	}
	err := os.Remove(filepath.Join(s.dir, id+reportSuffix))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("crash: acknowledge %q: %w", id, err)
	}
	return nil
}

// AcknowledgeAll deletes every pending crash report from s.
func (s *Store) AcknowledgeAll() error {
	if s == nil || s.dir == "" {
		return nil
	}
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("crash: read directory: %w", err)
	}
	var firstErr error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, reportSuffix) {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func pruneReports(dir string, limit int) error {
	if limit <= 0 {
		return nil
	}
	s := &Store{dir: dir}
	reports, err := s.PendingReports()
	if err != nil || len(reports) <= limit {
		return err
	}
	for _, r := range reports[:len(reports)-limit] {
		_ = s.Acknowledge(r.ID)
	}
	return nil
}

func readSessionMeta(path string) sessionMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionMeta{}
	}
	var m sessionMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return sessionMeta{}
	}
	return m
}

func readLimitedTrace(path string, limit int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > limit {
		return string(data[:limit]) + "\n... (truncated)\n", nil
	}
	return string(data), nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".crash-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// defaultCrashDir mirrors internal/engine.AppDirs so calling crash.Install
// with an empty Dir still writes under the platform's LogsDir/crashes.
func defaultCrashDir() string {
	id := core.App().ID
	name := id
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		name = "default"
	}
	home := os.Getenv("HOME")
	var logs string
	switch runtime.GOOS {
	case "android":
		if data := os.Getenv("NSGO_DATA_DIR"); data != "" {
			logs = filepath.Join(data, "logs")
		}
	case "darwin":
		if home != "" {
			logs = filepath.Join(home, "Library", "Logs", name)
		}
	case "ios":
		if home != "" {
			logs = filepath.Join(home, "Library", "Application Support", name, "logs")
		}
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			logs = filepath.Join(local, name, "Logs")
		}
	default:
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" && home != "" {
			state = filepath.Join(home, ".local", "state")
		}
		if state != "" {
			logs = filepath.Join(state, name)
		}
	}
	if logs == "" {
		return ""
	}
	return filepath.Join(logs, "crashes")
}
