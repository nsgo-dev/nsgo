//go:build android

package android

import (
	"context"
	"os"
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. A check of an APK the
// user installed reads the app's appcast through the runtime's HTTP, and
// package updater hands a verified APK to Android's package installer; one
// of an app a store installed checks nothing, as the store updates it.
var updateConfig updates.Config

// InstalledBy names the store that installed the app, which updates it,
// from the installing package the runtime passes in NSGO_INSTALLER
// (updates.AndroidStore).
func InstalledBy() string { return updates.AndroidStore(os.Getenv("NSGO_INSTALLER")) }

// withoutUpdateChecks answers the checks for updates of surface id's batch b, which Go carries out, and returns b without them, as the runtime knows no such item.
func (s *surface) withoutUpdateChecks(id uint32, b *driver.MountBatch) *driver.MountBatch {
	check := func(it driver.MountItem) bool { return it.Kind == driver.ItemCheckForUpdates }
	if !slices.ContainsFunc(b.Items, check) {
		return b
	}
	c := updateConfig
	for _, it := range b.Items {
		if check(it) {
			updates.Answer(it.Count, func(ctx context.Context) driver.Update { return updates.Find(ctx, c) },
				func(ev driver.Event) { s.dispatchTo(id, ev) })
		}
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), check)
	return &out
}

// withoutDesktopItems removes the desktop-only items Android has nothing for: task progress, which launchers have no portable surface for, the app icon's menu and recent documents. The Java protocol need not carry a command that has no effect.
func withoutDesktopItems(b *driver.MountBatch) *driver.MountBatch {
	desktop := func(it driver.MountItem) bool {
		return it.Kind == driver.ItemTaskProgress || it.Kind == driver.ItemDockMenu || it.Kind == driver.ItemRecentFile
	}
	if !slices.ContainsFunc(b.Items, desktop) {
		return b
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), desktop)
	return &out
}
