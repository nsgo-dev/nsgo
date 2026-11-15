package microphone

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"sync"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The recorder is what records for RecordWAV: Android's runtime, which the backend connects through internal/driver, or nsgotest's fake microphone. Its messages and reports are JSON; RecordingService.java and nsgotest/microphone.go speak them too.

// command is a message to the recorder.
type command struct {
	Op       string `json:"op"` // start, stop or authorize
	ID       uint64 `json:"id"`
	Name     string `json:"name,omitempty"`
	Rate     int    `json:"rate,omitempty"`
	Channels int    `json:"channels,omitempty"`
	// Level is the level interval in milliseconds, 0 for no levels.
	Level int64 `json:"level,omitempty"`
	Pause bool  `json:"pause,omitempty"`
}

// report is what the recorder reports: its state, as it connects and as it changes; a recording's start, levels, interruption and end, and the frames it wrote about once a second when it reports no levels; an earlier run's recording; and the answer to an authorize.
type report struct {
	Type      string  `json:"type"` // state, started, level, written, interrupted, ended, earlier or authorized
	ID        uint64  `json:"id"`
	Supported bool    `json:"supported"`
	Allowed   bool    `json:"allowed"`
	Path      string  `json:"path"`
	Rate      int     `json:"rate"`
	Channels  int     `json:"channels"`
	Frames    int64   `json:"frames"`
	Peak      float32 `json:"peak"`
	RMS       float32 `json:"rms"`
	Silenced  bool    `json:"silenced"`
	// Error is why a recording ended or a start or authorize failed, a code below; empty for none.
	Error  string `json:"error"`
	Detail string `json:"detail"`
}

var recorder struct {
	once       sync.Once
	mu         sync.Mutex
	nextID     uint64
	supported  bool
	allowed    bool
	recordings map[uint64]*Recording
	asks       map[uint64]chan error
	earlier    []*Recording
}

// reachable says a recorder is or will be connected: always on Android, whose backend connects as the app starts, and elsewhere while nsgotest's fake is.
func reachable() bool { return runtime.GOOS == "android" || driver.RecordingsConnected() }

// listen hears the recorder's reports from now on.
func listen() {
	recorder.once.Do(func() {
		recorder.mu.Lock()
		recorder.recordings = map[uint64]*Recording{}
		recorder.asks = map[uint64]chan error{}
		recorder.mu.Unlock()
		driver.HearRecordings(hear)
	})
}

func send(c command) {
	msg, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	driver.SendRecording(msg)
}

// hear takes one of the recorder's reports, on the goroutine that reports, which it never holds up: the Android backend's, which serves the app's views too.
func hear(data []byte) {
	var m report
	if json.Unmarshal(data, &m) != nil {
		return
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	switch m.Type {
	case "state":
		recorder.supported, recorder.allowed = m.Supported, m.Allowed
	case "authorized":
		if ask, ok := recorder.asks[m.ID]; ok {
			delete(recorder.asks, m.ID)
			ask <- reportError(m)
		}
	case "earlier":
		r := &Recording{started: make(chan struct{}), ended: make(chan struct{})}
		r.update(m)
		r.err = reportError(m)
		close(r.started)
		close(r.ended)
		recorder.earlier = append(recorder.earlier, r)
	default:
		r := recorder.recordings[m.ID]
		if r == nil {
			return
		}
		switch m.Type {
		case "started":
			r.mu.Lock()
			r.update(m)
			r.mu.Unlock()
			close(r.started)
		case "level":
			r.mu.Lock()
			r.frames, r.pending = max(r.frames, m.Frames), Level{Peak: m.Peak, RMS: m.RMS}
			r.mu.Unlock()
			if r.levels != nil {
				select {
				case r.levels <- struct{}{}:
				default:
				}
			}
		case "written":
			r.mu.Lock()
			r.frames = max(r.frames, m.Frames)
			r.mu.Unlock()
		case "interrupted":
			r.mu.Lock()
			r.interrupted = m.Silenced
			r.mu.Unlock()
		case "ended":
			delete(recorder.recordings, m.ID)
			r.mu.Lock()
			if m.Path != "" {
				r.update(m)
			}
			r.err, r.interrupted = reportError(m), false
			r.mu.Unlock()
			close(r.ended)
		}
	}
}

// update takes the file a report describes; r.mu is held or r not shared yet.
func (r *Recording) update(m report) {
	r.file = File{Path: m.Path, Rate: m.Rate, Channels: m.Channels}
	r.frames = m.Frames
}

// reportError is the error a report's code stands for.
func reportError(m report) error {
	var err error
	switch m.Error {
	case "":
		return nil
	case "denied":
		err = ErrDenied
	case "unavailable":
		err = ErrUnavailable
	case "busy":
		err = ErrBusy
	case "interrupted":
		err = ErrInterrupted
	case "full":
		err = ErrFull
	case "space":
		err = ErrNoSpace
	case "abandoned":
		err = ErrAbandoned
	case "partial":
		err = ErrPartial
	case "exists":
		return &fs.PathError{Op: "record", Path: m.Path, Err: fs.ErrExist}
	default:
		if m.Detail == "" {
			return errors.New("microphone: the recording failed")
		}
		return errors.New("microphone: " + m.Detail)
	}
	if m.Detail != "" {
		return fmt.Errorf("%w: %s", err, m.Detail)
	}
	return err
}

// recorderAllowed and the rest answer Allowed, Supported and Authorize through the recorder.
func recorderAllowed() bool {
	listen()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.allowed
}

func recorderSupported() bool {
	listen()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.supported
}

func recorderAuthorize() error {
	listen()
	answer := make(chan error, 1)
	recorder.mu.Lock()
	recorder.nextID++
	id := recorder.nextID
	recorder.asks[id] = answer
	recorder.mu.Unlock()
	send(command{Op: "authorize", ID: id})
	return <-answer
}
