//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

var (
	awakeCreate  func(objc.ID, uint32, objc.ID, *uint32) int32
	awakeRelease func(uint32) int32
)

type awakeGroup struct {
	assertion uint32
	refs      int
}

type awakeState struct {
	loaded bool
	ids    map[uint32]bool
	groups [2]awakeGroup
}

func (a *awakeState) load() {
	if a.loaded {
		return
	}
	lib, err := purego.Dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err == nil {
		purego.RegisterLibFunc(&awakeCreate, lib, "IOPMAssertionCreateWithName")
		purego.RegisterLibFunc(&awakeRelease, lib, "IOPMAssertionRelease")
	}
	a.loaded = true
	a.ids = map[uint32]bool{}
}

func (a *awakeState) take(id uint32, req driver.KeepAwake) {
	a.load()
	group := &a.groups[0]
	if req.Display {
		group = &a.groups[1]
	}
	if group.refs == 0 && awakeCreate != nil {
		name := "PreventUserIdleSystemSleep"
		if req.Display {
			name = "PreventUserIdleDisplaySleep"
		}
		_ = awakeCreate(apple.NSString(name), 255, apple.NSString(req.Reason), &group.assertion)
	}
	group.refs++
	a.ids[id] = req.Display
}

func (a *awakeState) release(id uint32) {
	display, ok := a.ids[id]
	if !ok {
		return
	}
	delete(a.ids, id)
	group := &a.groups[0]
	if display {
		group = &a.groups[1]
	}
	group.refs--
	if group.refs == 0 && group.assertion != 0 && awakeRelease != nil {
		_ = awakeRelease(group.assertion)
		group.assertion = 0
	}
}
