//go:build android

package android

import (
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// withoutPermissions answers b's permission requests from Go and returns b without them, as the runtime knows no such item: until it asks Android (docs/roadmap.md), what a phone keeps answers unknown and its settings do not open.
func (s *surface) withoutPermissions(b *driver.MountBatch) *driver.MountBatch {
	permission := func(it driver.MountItem) bool { return it.Kind == driver.ItemPermission }
	if !slices.ContainsFunc(b.Items, permission) {
		return b
	}
	for _, it := range b.Items {
		if permission(it) {
			driver.AnswerPermission(it.Count, *it.Permission, driver.PhonePermission, s.engine.Dispatch)
		}
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), permission)
	return &out
}
