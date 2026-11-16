//go:build ios

package ios

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/privacy"
)

// permission carries out an ItemPermission numbered id: privacy tells and asks, and the settings are the app's page in Settings. Nothing guides on a phone or a TV, so a guide answers the status at once.
func (s *surface) permission(id uint32, r driver.PermissionRequest) {
	e := s.engine
	answer := func(status int) { e.Dispatch(driver.Event{Kind: driver.EventPermission, Count: id, Index: status}) }
	switch r.Command {
	case driver.PermissionCheck, driver.PermissionGuide:
		answer(privacy.Status(r.Name))
	case driver.PermissionAsk:
		privacy.Request(r.Name, answer)
	case driver.PermissionOpenSettings:
		opened := func(ok bool) { e.Dispatch(driver.Event{Kind: driver.EventPermission, Count: id, Finished: ok}) }
		settings := apple.GoString(apple.Global(uikitPath, "UIApplicationOpenSettingsURLString"))
		// The app's page in Settings also holds its location and motion, which privacy does not tell.
		sensor := r.Name == driver.PermissionLocation || r.Name == driver.PermissionBackgroundLocation || r.Name == driver.PermissionMotion
		if settings == "" || !sensor && privacy.Status(r.Name) == driver.PermissionUnavailable {
			opened(false)
			return
		}
		openSystemURL(settings, opened)
	}
}
