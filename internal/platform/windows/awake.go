//go:build windows

package windows

import "github.com/nsgo-dev/nsgo/internal/driver"

var setThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")

type awakeState struct {
	ids             map[uint32]bool
	display, system int
}

func (a *awakeState) take(id uint32, req driver.KeepAwake) {
	if a.ids == nil {
		a.ids = map[uint32]bool{}
	}
	a.ids[id] = req.Display
	if req.Display {
		a.display++
	} else {
		a.system++
	}
	a.apply()
}

func (a *awakeState) release(id uint32) {
	display, ok := a.ids[id]
	if !ok {
		return
	}
	delete(a.ids, id)
	if display {
		a.display--
	} else {
		a.system--
	}
	a.apply()
}

func (a *awakeState) apply() {
	const continuous = 0x80000000
	var flags uintptr
	if a.display > 0 {
		flags |= 0x00000002
	}
	if a.system > 0 || a.display > 0 {
		flags |= 0x00000001
	}
	setThreadExecutionState.Call(continuous | flags)
}
