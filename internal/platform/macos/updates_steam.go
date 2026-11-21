//go:build darwin && !ios && nsgo_steam

package macos

import "github.com/nsgo-dev/nsgo/internal/driver"

const sparkleEnabled = false

func startSparkle() {}

func (s *surface) checkForUpdates(_ bool, id uint32) {
	s.engine.Dispatch(driver.Event{Kind: driver.EventUpdate, Count: id,
		Data: &driver.Update{Err: "updates: Steam updates the app"}})
}
