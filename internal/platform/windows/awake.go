//go:build windows

package windows

import (
	"sync"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var setThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")

// SetThreadExecutionState's flags, from winbase.h.
const (
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
	esContinuous      = 0x80000000
)

// awakeKey names a hold: the window that took it and the ID its tree gave it.
type awakeKey struct {
	w  *window
	id uint32
}

// awakeHold is a hold and the thread whose execution state carries it.
type awakeHold struct {
	display bool
	thread  uintptr
}

// awake is the process's keep-awake holds. The execution state SetThreadExecutionState sets is the calling thread's, and every window on a thread shares it: opened windows and panels share the app's UI thread, and a host's views share the thread that made them. So a window's holds are counted with those of every window on its thread, and each call sets that thread's state from all of them.
var awake struct {
	mu    sync.Mutex
	holds map[awakeKey]awakeHold
}

func (w *window) keepAwake(id uint32, req driver.KeepAwake) {
	thread, _, _ := getCurrentThreadID.Call()
	awake.mu.Lock()
	defer awake.mu.Unlock()
	if awake.holds == nil {
		awake.holds = map[awakeKey]awakeHold{}
	}
	awake.holds[awakeKey{w, id}] = awakeHold{display: req.Display, thread: thread}
	applyAwake(thread)
}

func (w *window) releaseAwake(id uint32) {
	awake.mu.Lock()
	defer awake.mu.Unlock()
	key := awakeKey{w, id}
	hold, ok := awake.holds[key]
	if !ok {
		return
	}
	delete(awake.holds, key)
	applyAwake(hold.thread)
}

// releaseAllAwake releases the window's holds as it is destroyed, on its thread: its tree's last releases never reach it.
func (w *window) releaseAllAwake() {
	awake.mu.Lock()
	defer awake.mu.Unlock()
	released := false
	for key := range awake.holds {
		if key.w == w {
			delete(awake.holds, key)
			released = true
		}
	}
	if released {
		thread, _, _ := getCurrentThreadID.Call()
		applyAwake(thread)
	}
}

// applyAwake sets the execution state of thread, the calling one, from the holds it carries; with none left it clears the requirements. awake.mu is held.
func applyAwake(thread uintptr) {
	flags := uintptr(esContinuous)
	for _, hold := range awake.holds {
		if hold.thread != thread {
			continue
		}
		flags |= esSystemRequired
		if hold.display {
			flags |= esDisplayRequired
		}
	}
	setThreadExecutionState.Call(flags)
}
