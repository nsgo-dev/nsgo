//go:build linux && !android

package linux

import "github.com/nsgo-dev/nsgo/internal/driver"

type awakeState struct {
	ids    map[uint32]bool
	cookie uint32
}

func (a *awakeState) take(id uint32, req driver.KeepAwake, appID string) {
	bindInputSources()
	if a.ids == nil {
		a.ids = map[uint32]bool{}
	}
	if len(a.ids) > 0 {
		a.ids[id] = req.Display
		return
	}
	bus := sessionBus()
	if bus == 0 {
		return
	}
	// ScreenSaver.Inhibit is implemented by GNOME, KDE and the desktop
	// portals. It prevents idle blanking; the platform ignores the reason
	// only where the session service does not expose an inhibitor list.
	reply := dbusCall(bus, "org.freedesktop.ScreenSaver", "/ScreenSaver", "org.freedesktop.ScreenSaver", "Inhibit",
		gVariantNewString(cstr(appID)), gVariantNewString(cstr(req.Reason)))
	if reply == 0 {
		return
	}
	child := gVariantGetChildValue(reply, 0)
	cookie := gVariantGetUint32(child)
	gVariantUnref(child)
	gVariantUnref(reply)
	if cookie != 0 {
		a.cookie = cookie
		a.ids[id] = req.Display
	}
}

func (a *awakeState) release(id uint32) {
	_, ok := a.ids[id]
	if !ok {
		return
	}
	delete(a.ids, id)
	bindInputSources()
	if len(a.ids) == 0 && a.cookie != 0 {
		if bus := sessionBus(); bus != 0 {
			gVariantUnrefNonZero(dbusCall(bus, "org.freedesktop.ScreenSaver", "/ScreenSaver", "org.freedesktop.ScreenSaver", "UnInhibit", gVariantNewUint32(a.cookie)))
		}
		a.cookie = 0
	}
}
