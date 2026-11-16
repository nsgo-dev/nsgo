//go:build android

package android

import (
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// withoutAuthSessions answers the auth sessions of surface id's batch b from Go, to that surface, and returns b without them, as the runtime knows no such item: until it opens Custom Tabs and an activity catches the app's scheme (docs/roadmap.md), a session ends unavailable.
func (s *surface) withoutAuthSessions(id uint32, b *driver.MountBatch) *driver.MountBatch {
	auth := func(it driver.MountItem) bool { return it.Kind == driver.ItemAuthSession }
	if !slices.ContainsFunc(b.Items, auth) {
		return b
	}
	for i := range b.Items {
		if ev, ok := driver.NoAuthSession(&b.Items[i]); ok {
			s.dispatchTo(id, ev)
		}
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), auth)
	return &out
}
