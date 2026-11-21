//go:build linux && !android && nsgo_steam

package linux

import "github.com/nsgo-dev/nsgo/internal/driver"

func (w *window) checkForUpdates(id uint32) {
	w.engine.Dispatch(driver.Event{Kind: driver.EventUpdate, Count: id,
		Data: &driver.Update{Err: "updates: Steam updates the app"}})
}
