//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// awakeKey names a hold: the surface that took it and the ID its tree gave it.
type awakeKey struct {
	s  *surface
	id uint32
}

// awake is the process's keep-awake holds, on the main thread. UIApplication's idle timer is the app's, not a surface's, so every surface's display holds count together, and the timer is set only as the first one comes and the last one goes: a computer-only hold, or a release while another display hold stays, leaves what the app or its host set alone.
var awake struct {
	holds   map[awakeKey]bool // whether each hold keeps the display on
	display int
}

func (s *surface) keepAwake(id uint32, req driver.KeepAwake) {
	key := awakeKey{s, id}
	if _, ok := awake.holds[key]; ok {
		return
	}
	if awake.holds == nil {
		awake.holds = map[awakeKey]bool{}
	}
	awake.holds[key] = req.Display
	if req.Display {
		awake.display++
		if awake.display == 1 {
			setIdleTimer(true)
		}
	}
}

func (s *surface) releaseAwake(id uint32) {
	key := awakeKey{s, id}
	display, ok := awake.holds[key]
	if !ok {
		return
	}
	delete(awake.holds, key)
	if display {
		awake.display--
		if awake.display == 0 {
			setIdleTimer(false)
		}
	}
}

// releaseAllAwake releases the surface's holds as it closes: its tree's last releases never reach it.
func (s *surface) releaseAllAwake() {
	for key := range awake.holds {
		if key.s == s {
			s.releaseAwake(key.id)
		}
	}
}

func setIdleTimer(disabled bool) {
	app := objc.ID(apple.Class("UIApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setIdleTimerDisabled:"), disabled)
}
