//go:build nsgo_steam

package updater

import "github.com/nsgo-dev/nsgo/core"

// Check does nothing in a Steam build, which Steam updates.
func Check(*core.Scope) {}

// CheckInBackground does nothing in a Steam build, which Steam updates.
func CheckInBackground(*core.Scope) {}

// CheckDaily does nothing in a Steam build, which Steam updates.
func CheckDaily(*core.Scope) {}
