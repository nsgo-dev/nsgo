//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func (s *surface) captureTouches(e *driver.Event) {
	windowPoint := func(p driver.Point) apple.CGPoint {
		return objc.Send[apple.CGPoint](s.native, sel("convertPoint:toView:"),
			apple.CGPoint{X: float64(p.X), Y: float64(p.Y)}, objc.ID(0))
	}
	s.mouse.Capture(e, func(p driver.Point) driver.NodeID {
		for n := s.hitAt(windowPoint(p)); n != 0; n = n.Send(selSuperview) {
			if v := views[n]; v != nil && v.surface == s {
				return v.id
			}
		}
		return 0
	}, func(id driver.NodeID) (driver.NodeID, driver.EventMask) {
		v := s.views[id]
		if v == nil {
			return 0, 0
		}
		var up driver.NodeID
		if v.up != nil {
			up = v.up.id
		}
		return up, v.events
	}, func(id driver.NodeID, p driver.Point) (driver.Point, bool) {
		v := s.views[id]
		if v == nil {
			return driver.Point{}, false
		}
		at := v.windowPoint(windowPoint(p))
		return driver.Point{X: driver.Points(at.X), Y: driver.Points(at.Y)}, true
	})
}
