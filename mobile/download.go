package mobile

import "github.com/nsgo-dev/nsgo/core"

// DownloadRequest describes a GET of URL, with optional HTTP headers.
type DownloadRequest = core.DownloadRequest

// DownloadOptions configures a background download. CertificatePEM, if
// provided, pins the server's exact PEM-encoded leaf certificate for this
// request, with its validity and hostname checked as for ordinary HTTPS.
type DownloadOptions = core.DownloadOptions

// DownloadTask identifies a download, including after the app restarts.
// Its methods send commands whose results arrive through OnUpdate.
type DownloadTask = core.DownloadTask

// DownloadState is the latest state of a background download.
type DownloadState = core.DownloadState

const (
	DownloadQueued      = core.DownloadQueued
	DownloadRunning     = core.DownloadRunning
	DownloadPaused      = core.DownloadPaused
	DownloadCompleted   = core.DownloadCompleted
	DownloadFailed      = core.DownloadFailed
	DownloadCanceled    = core.DownloadCanceled
	DownloadInterrupted = core.DownloadInterrupted
)

// DownloadProgress reports the task, destination, state and byte counts.
// Total is -1 until known. Error describes a failed or interrupted task.
type DownloadProgress = core.DownloadProgress

// ErrDownloadUnavailable is returned when the platform has no transfer host.
var ErrDownloadUnavailable = core.ErrDownloadUnavailable

// Download starts writing req to an absolute, app-accessible path. The
// task belongs to the app and keeps running when cx is disposed. It stages
// a partial file beside destination, then atomically publishes the result.
// Android apps enable android.background_downloads and start or resume a
// download while the app is visible.
func Download(cx *core.Scope, req DownloadRequest, destination string, opts DownloadOptions) (DownloadTask, error) {
	return cx.Download(req, destination, opts)
}

// OnDownloadUpdate reports saved and live transfers on cx's engine while
// the current owner lives. Completed reports survive app restarts until
// the app calls the task's Forget method.
func OnDownloadUpdate(cx *core.Scope, fn func(DownloadProgress)) { cx.OnDownloadUpdate(fn) }
