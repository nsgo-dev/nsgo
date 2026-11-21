package web

import (
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// screenError describes a browser API refusal or failure (typically a DOMException
// from Element.requestFullscreen, document.exitFullscreen or screen.orientation.lock).
type screenError struct {
	Name    string
	Message string
}

// screenHost abstracts the browser Fullscreen and Screen Orientation APIs so the
// state machine, gesture-deferral coordination and refusal classification can be
// verified with a fake browser in host unit tests as well as in real browsers.
type screenHost interface {
	canFullScreen() bool
	isFullScreen() bool
	requestFullScreen(settle func(*screenError)) (async bool, immediate *screenError)
	exitFullScreen(settle func(*screenError)) (async bool, immediate *screenError)

	canLockOrientation() bool
	currentOrientation() driver.Orientation
	lockOrientation(lockType string, settle func(*screenError)) (async bool, immediate *screenError)
	unlockOrientation() *screenError

	updateEnv()
	dispatch(ev driver.Event)
}

type fullScreenTransition struct {
	pending       bool
	on            bool
	reverse       bool
	legacyPending bool
	legacyOn      bool
}

type screenController struct {
	host             screenHost
	fs               fullScreenTransition
	pendingOrient    driver.Orientation
	hasPendingOrient bool
	orientSeq        uint64
	orientLocked     bool
	orientPending    bool
}

func newScreenController(host screenHost) *screenController {
	return &screenController{host: host}
}

func (c *screenController) setFullScreen(on bool) {
	f := &c.fs
	if f.pending {
		f.reverse = on != f.on
		return
	}
	if c.host.isFullScreen() == on {
		return
	}
	settled := false
	settle := func(err *screenError) {
		settled = true
		reverse := f.reverse
		pendingOrient, hasPendingOrient := c.pendingOrient, c.hasPendingOrient
		*f = fullScreenTransition{}
		c.pendingOrient, c.hasPendingOrient = 0, false
		if err != nil {
			c.reportFullScreenError(err)
			if hasPendingOrient {
				c.applyOrientation(pendingOrient)
			}
			c.host.updateEnv()
			return
		}
		c.host.updateEnv()
		if reverse {
			c.setFullScreen(!on)
			return
		}
		if hasPendingOrient {
			c.applyOrientation(pendingOrient)
		}
	}
	var (
		async     bool
		immediate *screenError
	)
	if on {
		async, immediate = c.host.requestFullScreen(settle)
	} else {
		async, immediate = c.host.exitFullScreen(settle)
	}
	if !async {
		if immediate != nil {
			c.reportFullScreenError(immediate)
			return
		}
		if !settled {
			f.legacyPending = true
			f.legacyOn = on
		}
		return
	}
	if !settled {
		f.pending, f.on = true, on
	}
}

func (c *screenController) onFullScreenChange() {
	f := &c.fs
	f.legacyPending = false
	c.host.updateEnv()
	if c.host.isFullScreen() && c.hasPendingOrient && !f.pending {
		mask := c.pendingOrient
		c.pendingOrient, c.hasPendingOrient = 0, false
		c.applyOrientation(mask)
	}
}

func (c *screenController) onFullScreenErrorEvent(err *screenError) {
	f := &c.fs
	// When a Promise-based transition is in flight, its rejection callback
	// reports the refusal once with the full DOMException details.
	if f.pending || !f.legacyPending {
		return
	}
	f.legacyPending = false
	if err == nil {
		err = &screenError{Name: "NotAllowedError", Message: "full screen request was refused"}
	}
	c.reportFullScreenError(err)
	if c.hasPendingOrient {
		mask := c.pendingOrient
		c.pendingOrient, c.hasPendingOrient = 0, false
		c.applyOrientation(mask)
	}
}

func (c *screenController) setOrientation(mask driver.Orientation) {
	c.orientSeq++
	f := &c.fs
	// Chrome on Android requires the document to be in full screen before
	// screen.orientation.lock resolves. When an app asks for both full screen
	// and an orientation lock in the same user gesture, hold the lock until
	// the full-screen entry settles.
	if mask != 0 && ((f.pending && f.on) || (f.legacyPending && f.legacyOn)) {
		c.pendingOrient = mask
		c.hasPendingOrient = true
		return
	}
	c.pendingOrient, c.hasPendingOrient = 0, false
	c.applyOrientation(mask)
}

func (c *screenController) applyOrientation(mask driver.Orientation) {
	seq := c.orientSeq
	if mask == 0 {
		if !c.orientLocked && !c.orientPending {
			return
		}
		c.orientLocked = false
		c.orientPending = false
		if err := c.host.unlockOrientation(); err != nil {
			c.reportOrientationError(err)
		}
		return
	}
	lockType := orientationLockType(mask, c.host.currentOrientation())
	settled := false
	async, immediate := c.host.lockOrientation(lockType, func(err *screenError) {
		settled = true
		if seq != c.orientSeq {
			// Superseded by a newer lock or unlock request (which may abort
			// the previous promise with AbortError).
			return
		}
		c.orientPending = false
		if err != nil {
			c.reportOrientationError(err)
			return
		}
		c.orientLocked = true
		c.host.updateEnv()
	})
	if !async {
		c.orientPending = false
		if immediate != nil {
			c.reportOrientationError(immediate)
			return
		}
		c.orientLocked = true
		c.host.updateEnv()
		return
	}
	if !settled {
		c.orientPending = true
	}
}

func (c *screenController) onOrientationChange() {
	c.host.updateEnv()
}

func (c *screenController) reportFullScreenError(err *screenError) {
	reason, msg := classifyRefusal(err, "full screen request was refused")
	c.host.dispatch(driver.Event{Kind: driver.EventFullScreenError, Index: reason, Text: msg})
}

func (c *screenController) reportOrientationError(err *screenError) {
	reason, msg := classifyRefusal(err, "orientation lock request was refused")
	c.host.dispatch(driver.Event{Kind: driver.EventOrientationError, Index: reason, Text: msg})
}

// orientationLockType maps a driver.Orientation bitmask to a W3C Screen
// Orientation OrientationLockType string, matching Android's mixed-mask rule
// by keeping the current orientation when allowed or choosing an allowed one
// on the same axis.
func orientationLockType(mask, current driver.Orientation) string {
	const (
		portrait      = driver.OrientationPortrait
		upsideDown    = driver.OrientationPortraitUpsideDown
		left          = driver.OrientationLandscapeLeft
		right         = driver.OrientationLandscapeRight
		bothPortrait  = portrait | upsideDown
		bothLandscape = driver.OrientationLandscape
		all           = bothPortrait | bothLandscape
	)
	mask &= all
	switch mask {
	case all:
		return "any"
	case bothPortrait:
		return "portrait"
	case bothLandscape:
		return "landscape"
	case portrait:
		return "portrait-primary"
	case upsideDown:
		return "portrait-secondary"
	case left:
		return "landscape-primary"
	case right:
		return "landscape-secondary"
	}
	chosen := driver.Orientation(0)
	if current != 0 && current&mask != 0 {
		chosen = current
	} else {
		var sameAxis driver.Orientation
		switch {
		case current&bothPortrait != 0:
			sameAxis = mask & bothPortrait
		case current&bothLandscape != 0:
			sameAxis = mask & bothLandscape
		}
		pick := sameAxis
		if pick == 0 {
			pick = mask
		}
		for _, bit := range []driver.Orientation{portrait, upsideDown, left, right} {
			if pick&bit != 0 {
				chosen = bit
				break
			}
		}
	}
	switch chosen {
	case portrait:
		return "portrait-primary"
	case upsideDown:
		return "portrait-secondary"
	case left:
		return "landscape-primary"
	case right:
		return "landscape-secondary"
	default:
		return "any"
	}
}

// parseScreenOrientation maps W3C screen.orientation.type values to driver.Orientation.
func parseScreenOrientation(typ string) driver.Orientation {
	switch typ {
	case "portrait-primary":
		return driver.OrientationPortrait
	case "portrait-secondary":
		return driver.OrientationPortraitUpsideDown
	case "landscape-primary":
		return driver.OrientationLandscapeLeft
	case "landscape-secondary":
		return driver.OrientationLandscapeRight
	default:
		return 0
	}
}

// parseLegacyOrientation maps legacy window.orientation angles (0, 90, -90, 180)
// to driver.Orientation for older Safari builds without screen.orientation.
func parseLegacyOrientation(angle int) driver.Orientation {
	switch angle {
	case 0:
		return driver.OrientationPortrait
	case 180, -180:
		return driver.OrientationPortraitUpsideDown
	case 90:
		return driver.OrientationLandscapeLeft
	case -90, 270:
		return driver.OrientationLandscapeRight
	default:
		return 0
	}
}

// classifyRefusal maps a browser DOMException (Name + Message) to a
// driver.Refusal* code and a readable message.
func classifyRefusal(err *screenError, fallback string) (int, string) {
	if err == nil {
		return driver.RefusalOther, fallback
	}
	msg := strings.TrimSpace(err.Message)
	if msg == "" {
		msg = strings.TrimSpace(err.Name)
	}
	if msg == "" {
		msg = fallback
	}
	switch err.Name {
	case "NotSupportedError":
		return driver.RefusalUnavailable, msg
	case "NotAllowedError", "SecurityError":
		return driver.RefusalDenied, msg
	case "TypeError":
		lower := strings.ToLower(msg)
		for _, hint := range []string{"gesture", "permission", "denied", "allowed", "activation", "disallowed", "policy"} {
			if strings.Contains(lower, hint) {
				return driver.RefusalDenied, msg
			}
		}
	}
	return driver.RefusalOther, msg
}
