package driver

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Sentinel errors shared between package cloud, core, platform backends and
// nsgotest's in-memory cloud store.
var (
	// ErrCloudUnavailable reports that cloud storage is unavailable on this
	// platform or the user is not signed in to an account; writes are never
	// silently dropped.
	ErrCloudUnavailable = errors.New("cloud: storage unavailable")
	// ErrCloudNotFound reports that a requested key or file does not exist in
	// cloud storage.
	ErrCloudNotFound = errors.New("cloud: not found")
	// ErrCloudConflict reports that a cloud file or key has conflicting local
	// and remote versions that must be resolved.
	ErrCloudConflict = errors.New("cloud: conflict")
)

// CloudConflictError describes a conflict between local and remote versions of
// a cloud key or file. It unwraps to ErrCloudConflict.
type CloudConflictError struct {
	Key        string
	Local      []byte
	Remote     []byte
	LocalTime  time.Time
	RemoteTime time.Time
}

func (e *CloudConflictError) Error() string {
	return fmt.Sprintf("cloud: conflict on %q", e.Key)
}

func (e *CloudConflictError) Unwrap() error { return ErrCloudConflict }

// CloudChangeReason explains why an external cloud storage change arrived.
type CloudChangeReason uint8

const (
	// CloudChangeServer reports data updated from another device or the server.
	CloudChangeServer CloudChangeReason = iota
	// CloudChangeInitialSync reports values downloaded on initial sync after install or sign-in.
	CloudChangeInitialSync
	// CloudChangeQuotaViolation reports that the cloud key-value store exceeded its quota.
	CloudChangeQuotaViolation
	// CloudChangeAccount reports that the signed-in cloud account changed.
	CloudChangeAccount
)

// String returns a human-readable name for the change reason.
func (r CloudChangeReason) String() string {
	switch r {
	case CloudChangeInitialSync:
		return "initial_sync"
	case CloudChangeQuotaViolation:
		return "quota_violation"
	case CloudChangeAccount:
		return "account"
	default:
		return "server"
	}
}

// CloudChangeEvent describes keys and files changed externally in cloud storage.
type CloudChangeEvent struct {
	Keys   []string
	Files  []string
	Reason CloudChangeReason
}

// CloudConflict describes a conflicting remote version of a cloud file or key
// and provides Resolve to commit the winning bytes.
type CloudConflict struct {
	Name       string
	Local      []byte
	Remote     []byte
	LocalTime  time.Time
	RemoteTime time.Time
	Resolve    func(chosen []byte) error
}

// CloudStore is an optional per-app cloud storage backend (core.RootOptions.Cloud),
// used by nsgotest to give each test app an isolated in-memory cloud store.
// When nil, package cloud uses the native platform backend.
type CloudStore interface {
	Available() bool
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
	Keys() ([]string, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte) error
	DeleteFile(name string) error
	ListFiles() ([]string, error)
	WatchChanges(onChange func(CloudChangeEvent)) (stop func())
	OnConflict(resolve func(CloudConflict)) (stop func())
}

// cloudBridge is how package cloud reaches the Android runtime's Google Play
// Games saved games bridge outside any surface, mirroring Recordings and
// LiveUpdates: JSON commands wait until the backend connects, and reports wait
// for package cloud's handler.
var cloudBridge struct {
	sync.Mutex
	send    func([]byte)
	queue   [][]byte
	hear    func([]byte)
	reports [][]byte
}

// SendCloudStorage sends package cloud's msg to the connected Android runtime.
func SendCloudStorage(msg []byte) {
	cloudBridge.Lock()
	send := cloudBridge.send
	if send == nil {
		cloudBridge.queue = append(cloudBridge.queue, msg)
	}
	cloudBridge.Unlock()
	if send != nil {
		send(msg)
	}
}

// ConnectCloudStorage registers the Android backend's sender for package cloud
// messages, draining any queued messages first. A nil send disconnects it.
func ConnectCloudStorage(send func([]byte)) {
	cloudBridge.Lock()
	var queue [][]byte
	if send != nil {
		queue, cloudBridge.queue = cloudBridge.queue, nil
	}
	cloudBridge.send = send
	cloudBridge.Unlock()
	for _, msg := range queue {
		send(msg)
	}
}

// CloudStorageConnected reports whether a cloud storage runtime bridge is connected.
func CloudStorageConnected() bool {
	cloudBridge.Lock()
	defer cloudBridge.Unlock()
	return cloudBridge.send != nil
}

// HearCloudStorage registers package cloud's handler for runtime reports,
// delivering any queued reports first.
func HearCloudStorage(fn func([]byte)) {
	cloudBridge.Lock()
	reports := cloudBridge.reports
	cloudBridge.hear, cloudBridge.reports = fn, nil
	cloudBridge.Unlock()
	for _, r := range reports {
		fn(r)
	}
}

// ReportCloudStorage hands a runtime report to package cloud.
func ReportCloudStorage(report []byte) {
	cloudBridge.Lock()
	hear := cloudBridge.hear
	if hear == nil {
		cloudBridge.reports = append(cloudBridge.reports, report)
	}
	cloudBridge.Unlock()
	if hear != nil {
		hear(report)
	}
}
