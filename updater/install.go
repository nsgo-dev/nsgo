package updater

import "github.com/nsgo-dev/nsgo/core"

// Install downloads' verified package through the platform's installer.
// Linux asks its package manager for elevation; Android opens the package
// installer. The callback runs on the app's graph goroutine.
func Install(cx *core.Scope, path string, done func(error)) {
	core.InstallUpdate(cx, path, done)
}
