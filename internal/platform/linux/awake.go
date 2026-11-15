//go:build linux && !android

package linux

import (
	"log"
	"os"
	"path/filepath"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Keep-awake holds. A display hold inhibits the session's idling with org.freedesktop.ScreenSaver.Inhibit, which GNOME and KDE serve, so the screen does not blank and the computer does not idle-sleep. A computer-only hold inhibits only suspend, and the display still blanks: an unsandboxed app on GNOME asks org.gnome.SessionManager.Inhibit with its Suspend flag itself, since GNOME's portal backend hands the session manager the empty ID the portal knows such an app by, which it refuses while the portal answers that the hold was taken; any other app asks the desktop portal's Inhibit with its Suspend flag, which KDE's backend serves, and without the portal falls back on the ScreenSaver inhibitor, which keeps the display on too. GNOME's inhibitors refuse an empty app ID and an empty reason, so a hold names an app built without nsgo.yaml's id by its name, else its executable, and gives the ID as the reason when the app gives none. A hold the session refuses is logged. The calls go to the bus on portalQueue's thread, in order: the bus starts the portal as the first call reaches it, which can take seconds.

const (
	screenSaverName     = "org.freedesktop.ScreenSaver"
	screenSaverPath     = "/ScreenSaver"
	screenSaverIface    = "org.freedesktop.ScreenSaver"
	sessionManagerName  = "org.gnome.SessionManager"
	sessionManagerPath  = "/org/gnome/SessionManager"
	sessionManagerIface = "org.gnome.SessionManager"
	inhibitIface        = "org.freedesktop.portal.Inhibit"
	inhibitSuspend      = 4 // the Suspend flag of the portal's Inhibit and of the session manager's
)

// awakeVia is the service a hold's inhibition goes to.
type awakeVia int

const (
	viaScreenSaver    awakeVia = iota // org.freedesktop.ScreenSaver: the session does not idle
	viaSessionManager                 // org.gnome.SessionManager with the Suspend flag
	viaPortal                         // the desktop portal's Inhibit with the Suspend flag
)

// awakeService picks the service for a hold: a display hold inhibits the screen saver, a computer-only hold of an unsandboxed app the session manager where GNOME's runs on the session bus, and any other the portal.
func awakeService(display, sandboxed, gnomeSession bool) awakeVia {
	switch {
	case display:
		return viaScreenSaver
	case !sandboxed && gnomeSession:
		return viaSessionManager
	}
	return viaPortal
}

// sandboxed reports whether the app runs in a Flatpak or a Snap, whose ID the portal finds itself.
func sandboxed() bool { return InstalledBy() != "" }

// awakeAppID is the ID a hold names the app by: nsgo.yaml's id, else the app's name, else its executable's base name.
func awakeAppID(id, name, exe string) string {
	if id != "" {
		return id
	}
	if name != "" {
		return name
	}
	if base := filepath.Base(exe); exe != "" && base != "/" && base != "." {
		return base
	}
	return "nsgo"
}

// awakeReason is the reason a hold gives, the app's ID when the app gives none.
func awakeReason(reason, appID string) string {
	if reason != "" {
		return reason
	}
	return appID
}

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
	exe, _ := os.Executable()
	id := awakeAppID(appID, core.App().Name, exe)
	reason := awakeReason(req.Reason, id)
	bus := sessionBus()
	box := sandboxed()
	gnome := bus != 0 && !box && !req.Display && hasNameOwner(bus, sessionManagerName)
	switch awakeService(req.Display, box, gnome) {
	case viaSessionManager:
		return inhibitWith(bus, sessionManagerName, sessionManagerPath, sessionManagerIface, "Inhibit", "Uninhibit",
			gVariantNewString(cstr(id)), gVariantNewUint32(0), gVariantNewString(cstr(reason)), gVariantNewUint32(inhibitSuspend))
	case viaPortal:
		// The portal's bus registers the app by nsgo.yaml's id alone: the registry refuses any other name.
		if pbus := portalBus(appID); pbus != 0 {
			reply := portalCall(pbus, globalShortcutPath, inhibitIface, "Inhibit", gVariantNewString(cstr("")), gVariantNewUint32(inhibitSuspend),
				portalVariant("a{sv}", "{'reason': <"+portalQuote(reason)+">}"))
			if reply != 0 {
				handle := portalTupleText(reply, 0)
				gVariantUnref(reply)
				if handle != "" {
					return func() { gVariantUnrefNonZero(portalCall(pbus, handle, globalRequestIface, "Close")) }
				}
			}
		}
	}
	if bus == 0 {
		log.Print("nsgo: keep awake: no session bus")
		return nil
	}
	return inhibitWith(bus, screenSaverName, screenSaverPath, screenSaverIface, "Inhibit", "UnInhibit",
		gVariantNewString(cstr(id)), gVariantNewString(cstr(reason)))
}

// inhibitWith calls an inhibitor's method, which answers a cookie, with args, and returns what releases the cookie with its release method; it logs a refusal and returns nil.
func inhibitWith(bus uintptr, name, path, iface, method, release string, args ...uintptr) func() {
	reply, err := dbusTry(bus, name, path, iface, method, args...)
	if err != nil {
		log.Printf("nsgo: keep awake: %s.%s: %v", iface, method, err)
		return nil
	}
	cookie := portalTupleUint32(reply, 0)
	gVariantUnref(reply)
	return func() {
		gVariantUnrefNonZero(dbusCall(bus, name, path, iface, release, gVariantNewUint32(cookie)))
	}
}

// hasNameOwner reports whether a name has an owner on the bus, without starting a service that would take it.
func hasNameOwner(bus uintptr, name string) bool {
	reply := dbusCall(bus, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "GetNameOwner", gVariantNewString(cstr(name)))
	if reply == 0 {
		return false
	}
	gVariantUnref(reply)
	return true
}
