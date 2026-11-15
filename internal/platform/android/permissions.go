//go:build android

package android

import (
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// withoutPermissions answers the permission requests of surface id's batch b from Go, to that surface, and returns b without them, as the runtime knows no such item: until it asks Android (docs/roadmap.md), what a phone keeps answers unknown and its settings do not open.
func (s *surface) withoutPermissions(id uint32, b *driver.MountBatch) *driver.MountBatch {
	permission := func(it driver.MountItem) bool { return it.Kind == driver.ItemPermission }
	if !slices.ContainsFunc(b.Items, permission) {
		return b
	}
	for _, it := range b.Items {
		if permission(it) {
			driver.AnswerPermission(it.Count, *it.Permission, driver.PhonePermission, func(ev driver.Event) { s.dispatchTo(id, ev) })
		}
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), permission)
	return &out
}
