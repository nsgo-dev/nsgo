//go:build linux && !android

package linux

import "github.com/nsgo-dev/nsgo/internal/driver"

// Keep-awake holds. A display hold inhibits the session's idling with org.freedesktop.ScreenSaver.Inhibit, which GNOME and KDE serve, so the screen does not blank and the computer does not idle-sleep. A computer-only hold inhibits only suspend, through the desktop portal's Inhibit with its Suspend flag, and the display still blanks; without the portal it falls back on the ScreenSaver inhibitor, which keeps the display on too. The calls go to the bus on portalQueue's thread, in order: the bus starts the portal as the first call reaches it, which can take seconds.

const (
	screenSaverName  = "org.freedesktop.ScreenSaver"
	screenSaverPath  = "/ScreenSaver"
	screenSaverIface = "org.freedesktop.ScreenSaver"
	inhibitIface     = "org.freedesktop.portal.Inhibit"
	inhibitSuspend   = 4 // Inhibit's Suspend flag
)

// awakeKey names a hold: the window that took it and the ID its tree gave it.
type awakeKey struct {
	w  *window
	id uint32
}

// awakeGroup is the holds that share one inhibition: a window's tree numbers its holds alone, and every window's holds of one kind share it.
type awakeGroup struct {
	refs int
	end  func() // ends the group's inhibition, nil without one
}

// awake is the process's keep-awake holds, on portalQueue's thread.
var awake struct {
	holds  map[awakeKey]bool // whether each hold keeps the display on
	groups [2]awakeGroup     // the computer-only holds and the display holds
}

func (w *window) keepAwake(id uint32, req driver.KeepAwake) {
	if w.closing {
		return
	}
	bindGlobalHotKeys()
	key, appID := awakeKey{w, id}, w.opts.ID
	portalQueue() <- func() {
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
			g.end = inhibitAwake(appID, req)
		}
	}
}

func (w *window) releaseAwake(id uint32) {
	key := awakeKey{w, id}
	portalQueue() <- func() { releaseAwakeHold(key) }
}

// releaseAllAwake releases the window's holds as it closes: its tree's last releases never reach it.
func (w *window) releaseAllAwake() {
	portalQueue() <- func() {
		for key := range awake.holds {
			if key.w == w {
				releaseAwakeHold(key)
			}
		}
	}
}

func releaseAwakeHold(key awakeKey) {
	display, ok := awake.holds[key]
	if !ok {
		return
	}
	delete(awake.holds, key)
	g := awakeGroupOf(display)
	g.refs--
	if g.refs == 0 && g.end != nil {
		g.end()
		g.end = nil
	}
}

func awakeGroupOf(display bool) *awakeGroup {
	if display {
		return &awake.groups[1]
	}
	return &awake.groups[0]
}

// inhibitAwake starts the inhibition req asks for and returns what ends it, nil when none started.
func inhibitAwake(appID string, req driver.KeepAwake) func() {
	if !req.Display {
		if bus := portalBus(appID); bus != 0 {
			reply := portalCall(bus, globalShortcutPath, inhibitIface, "Inhibit", gVariantNewString(cstr("")), gVariantNewUint32(inhibitSuspend),
				portalVariant("a{sv}", "{'reason': <"+portalQuote(req.Reason)+">}"))
			if reply != 0 {
				handle := portalTupleText(reply, 0)
				gVariantUnref(reply)
				if handle != "" {
					return func() { gVariantUnrefNonZero(portalCall(bus, handle, globalRequestIface, "Close")) }
				}
			}
		}
	}
	bus := sessionBus()
	if bus == 0 {
		return nil
	}
	reply := dbusCall(bus, screenSaverName, screenSaverPath, screenSaverIface, "Inhibit", gVariantNewString(cstr(appID)), gVariantNewString(cstr(req.Reason)))
	if reply == 0 {
		return nil
	}
	cookie := portalTupleUint32(reply, 0)
	gVariantUnref(reply)
	return func() {
		gVariantUnrefNonZero(dbusCall(bus, screenSaverName, screenSaverPath, screenSaverIface, "UnInhibit", gVariantNewUint32(cookie)))
	}
}
