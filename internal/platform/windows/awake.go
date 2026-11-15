//go:build windows

package windows

import (
	"log"
	"sync"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	powerCreateRequest = kernel32.NewProc("PowerCreateRequest")
	powerSetRequest    = kernel32.NewProc("PowerSetRequest")
	powerClearRequest  = kernel32.NewProc("PowerClearRequest")
)

// POWER_REQUEST_TYPE's values and REASON_CONTEXT's version and simple-string flag, from winnt.h.
const (
	powerRequestDisplayRequired     = 0
	powerRequestSystemRequired      = 1
	powerRequestContextVersion      = 0
	powerRequestContextSimpleString = 0x1
)

// reasonContext is REASON_CONTEXT carrying a simple string: Version, Flags, then the union of SimpleReasonString and Detailed, whose first member reason overlays and whose other members the blank fields stand for, so the struct has C's size and offsets on 32- and 64-bit Windows.
type reasonContext struct {
	version uint32
	flags   uint32
	reason  *uint16
	_       [2]uint32 // Detailed.LocalizedReasonId and ReasonStringCount
	_       uintptr   // Detailed.ReasonStrings
}

// awakeKey names a hold: the window that took it and the ID its tree gave it.
type awakeKey struct {
	w  *window
	id uint32
}

// awakeGroup is the holds that share one power request: a window's tree numbers its holds alone, and every window's holds of one kind share it.
type awakeGroup struct {
	refs    int
	request uintptr // the power request, 0 without one
}

// awake is the process's keep-awake holds. Each kind of hold is one power request, which powercfg /requests lists with the reason of the hold that created it: PowerRequestSystemRequired for a computer-only hold, with PowerRequestDisplayRequired as well for a display hold. PowerRequestExecutionRequired is left alone, as it keeps a packaged app's process from being suspended rather than the computer from sleeping, and implies PowerRequestSystemRequired only where the system has no connected standby. Windows on several threads take and release holds, so the state is locked.
var awake struct {
	mu     sync.Mutex
	holds  map[awakeKey]bool // whether each hold keeps the display on
	groups [2]awakeGroup     // the computer-only holds and the display holds
}

func (w *window) keepAwake(id uint32, req driver.KeepAwake) {
	awake.mu.Lock()
	defer awake.mu.Unlock()
	key := awakeKey{w, id}
	if _, ok := awake.holds[key]; ok {
		return
	}
	if awake.holds == nil {
		awake.holds = map[awakeKey]bool{}
	}
	awake.holds[key] = req.Display
	g := awakeGroupOf(req.Display)
	g.refs++
	if g.refs == 1 {
		g.request = createPowerRequest(req)
	}
}

func (w *window) releaseAwake(id uint32) {
	awake.mu.Lock()
	defer awake.mu.Unlock()
	releaseAwakeHold(awakeKey{w, id})
}

// releaseAllAwake releases the window's holds as it is destroyed: its tree's last releases never reach it.
func (w *window) releaseAllAwake() {
	awake.mu.Lock()
	defer awake.mu.Unlock()
	for key := range awake.holds {
		if key.w == w {
			releaseAwakeHold(key)
		}
	}
}

// releaseAwakeHold releases one hold, and its group's power request with the group's last hold. awake.mu is held.
func releaseAwakeHold(key awakeKey) {
	display, ok := awake.holds[key]
	if !ok {
		return
	}
	delete(awake.holds, key)
	g := awakeGroupOf(display)
	g.refs--
	if g.refs == 0 && g.request != 0 {
		for _, kind := range powerRequestKinds(display) {
			powerClearRequest.Call(g.request, kind)
		}
		closeHandle.Call(g.request)
		g.request = 0
	}
}

func awakeGroupOf(display bool) *awakeGroup {
	if display {
		return &awake.groups[1]
	}
	return &awake.groups[0]
}

// powerRequestKinds is the request types a hold sets.
func powerRequestKinds(display bool) []uintptr {
	if display {
		return []uintptr{powerRequestSystemRequired, powerRequestDisplayRequired}
	}
	return []uintptr{powerRequestSystemRequired}
}

// createPowerRequest creates a power request carrying req's reason and sets the types req asks for; it logs a failure and returns 0 when it created none.
func createPowerRequest(req driver.KeepAwake) uintptr {
	ctx := reasonContext{version: powerRequestContextVersion, flags: powerRequestContextSimpleString, reason: wide(req.Reason)}
	h, _, err := powerCreateRequest.Call(uintptr(unsafe.Pointer(&ctx)))
	if h == 0 || h == uintptr(syscall.InvalidHandle) {
		log.Printf("nsgo: keep awake: PowerCreateRequest: %v", err)
		return 0
	}
	for _, kind := range powerRequestKinds(req.Display) {
		if r, _, err := powerSetRequest.Call(h, kind); r == 0 {
			log.Printf("nsgo: keep awake: PowerSetRequest(%d): %v", kind, err)
		}
	}
	return h
}
