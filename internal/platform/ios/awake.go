//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

type awakeState struct {
	ids     map[uint32]bool
	display int
}

func (a *awakeState) take(id uint32, req driver.KeepAwake) {
	if a.ids == nil {
		a.ids = map[uint32]bool{}
	}
	a.ids[id] = req.Display
	if req.Display {
		a.display++
	}
	setIdleTimer(a.display > 0)
}

func (a *awakeState) release(id uint32) {
	display, ok := a.ids[id]
	if !ok {
		return
	}
	delete(a.ids, id)
	if display {
		a.display--
	}
	setIdleTimer(a.display > 0)
}

func setIdleTimer(disabled bool) {
	app := objc.ID(apple.Class("UIApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setIdleTimerDisabled:"), disabled)
}
