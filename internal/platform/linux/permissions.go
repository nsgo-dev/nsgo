//go:build linux && !android

package linux

import "github.com/nsgo-dev/nsgo/internal/driver"

// linuxPermission is whether Linux keeps the permission name from the app: nothing outside a sandbox, and inside Flatpak or Snap the microphone and the camera, which their portals give and which are not read yet (docs/roadmap.md).
func linuxPermission(name string) int {
	if InstalledBy() != "" && (name == driver.PermissionMicrophone || name == driver.PermissionCamera) {
		return driver.PermissionUnknown
	}
	return driver.PermissionNotGated
}
