//go:build darwin && !ios

package macos

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/privacy"
)

// permission carries out an ItemPermission numbered id: privacy tells and asks, System Settings opens at the permission's pane, and the guide follows it (permissionguide.go).
func (s *surface) permission(id uint32, r driver.PermissionRequest) {
	e := s.engine
	answer := func(status int) { e.Dispatch(driver.Event{Kind: driver.EventPermission, Count: id, Index: status}) }
	switch r.Command {
	case driver.PermissionCheck:
		answer(privacy.Status(r.Name))
	case driver.PermissionAsk:
		privacy.Request(r.Name, answer)
	case driver.PermissionOpenSettings:
		u := privacySettingsURL(r.Name)
		e.Dispatch(driver.Event{Kind: driver.EventPermission, Count: id, Finished: u != "" && openSystemURL(u)})
	case driver.PermissionGuide:
		s.guidePermission(id, r)
	case driver.PermissionStopGuide:
		stopGuide(s, id)
	}
}
