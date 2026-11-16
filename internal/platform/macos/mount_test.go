//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestMountIgnoresCommandsForMissingViews(t *testing.T) {
	onMain(func() {
		loadAppKit()
		s := &surface{native: objc.ID(classView).Send(selNew), animator: driver.NewAnimator(nil)}
		defer s.native.Send(selRelease)
		root := s.create(driver.KindView, 1)
		defer root.native.Send(selRelease)
		defer delete(views, root.native)
		s.views = []*view{nil, root, nil}
		var items []driver.MountItem
		for _, id := range []driver.NodeID{0, 2, 1 << 30} {
			for _, kind := range []driver.ItemKind{driver.ItemFocus, driver.ItemBlur, driver.ItemScrollTo, driver.ItemScrollBy, driver.ItemZoomTo} {
				items = append(items, driver.MountItem{Kind: kind, ID: id})
			}
		}
		s.apply(&driver.MountBatch{Root: 1, Items: items})
		if root.native.Send(selSuperview) != s.native {
			t.Error("valid root was not mounted after ignoring invalid commands")
		}
	})
}
