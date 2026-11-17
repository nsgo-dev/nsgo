//go:build linux && !android

package backend

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/linux"
)

func run(root engine.App, app func(*core.Scope), o Options) {
	updates := updateConfig()
	updates.Store = linux.InstalledBy()
	root, app = withUpdateChecks(root, app, updates, false)
	linux.Run(rootOf(root, app), linux.Options{Title: name, Name: core.App().Name, Width: o.Width, Height: o.Height, Hidden: app != nil, ID: id,
		Components: o.Components, Constraints: o.Constraints, Fonts: o.Fonts, Appearance: o.Appearance, Perf: o.Monitor,
		Transparent: o.Transparent, NoShadow: o.NoShadow, SkipTaskbar: o.SkipTaskbar,
		Updates: updates, InstanceKey: o.InstanceKey})
}
