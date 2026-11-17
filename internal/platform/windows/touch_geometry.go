//go:build windows

package windows

import "github.com/nsgo-dev/nsgo/internal/driver"

func (w *window) captureTouches(e *driver.Event) {
	w.mouse.Capture(e, func(p driver.Point) driver.NodeID {
		if root := w.view(w.root); root != nil {
			if v := hit(root, p, func(*view) bool { return true }); v != nil {
				return v.id
			}
		}
		return 0
	}, func(id driver.NodeID) (driver.NodeID, driver.EventMask) {
		v := w.view(id)
		if v == nil {
			return 0, 0
		}
		var up driver.NodeID
		if v.parent != nil {
			up = v.parent.id
		}
		return up, v.events
	}, func(id driver.NodeID, p driver.Point) (driver.Point, bool) {
		if v := w.view(id); v != nil {
			return v.surfacePoint(p), true
		}
		return driver.Point{}, false
	})
}
