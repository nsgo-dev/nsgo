//go:build js && wasm

package harmonyos

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestPressSkipsRowsOutsideTheScrollViewport presses a tab bar laid out below a scroll view whose content runs on under it, as a phone's index above a drawn tab bar does: the row under the tab is clipped by the scroll view, so the tab takes the press, while a row inside the viewport still does.
func TestPressSkipsRowsOutsideTheScrollViewport(t *testing.T) {
	m := &mounter{views: map[driver.NodeID]*view{}}
	add := func(id, parent driver.NodeID, kind driver.ViewKind, x, y, w, h driver.Points, pressable bool) {
		m.order++
		m.views[id] = &view{id: id, parent: parent, kind: kind, order: m.order, pressable: pressable,
			frame: driver.Bounds{Origin: driver.Point{X: x, Y: y}, Size: driver.Size{Width: w, Height: h}}}
	}
	add(1, 0, driver.KindView, 0, 0, 400, 800, false)
	add(2, 1, driver.KindScroll, 0, 0, 400, 700, false)
	add(3, 2, driver.KindView, 0, 0, 400, 2000, false)
	add(4, 3, driver.KindView, 0, 600, 400, 80, true)
	add(5, 3, driver.KindView, 0, 690, 400, 80, true)
	add(6, 1, driver.KindView, 0, 700, 400, 100, false)
	add(7, 6, driver.KindView, 0, 0, 200, 100, true)
	if got := m.pressTarget(driver.Point{X: 100, Y: 740}); got != 7 {
		t.Fatalf("a press on the tab went to %d, want 7", got)
	}
	if got := m.pressTarget(driver.Point{X: 100, Y: 640}); got != 4 {
		t.Fatalf("a press on the visible row went to %d, want 4", got)
	}
	// Scrolled, the second row is in the viewport and takes the press.
	m.views[2].offset = driver.Point{Y: 100}
	if got := m.pressTarget(driver.Point{X: 100, Y: 640}); got != 5 {
		t.Fatalf("a press on the scrolled row went to %d, want 5", got)
	}
}
