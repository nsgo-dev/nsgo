//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func TestHitTestsFollowLayerTransforms(t *testing.T) {
	onMain(func() {
		loadAppKit()
		s := &surface{}
		makeView := func(parent *view, f driver.Bounds, tr driver.Transform) *view {
			n := objc.ID(classView).Send(selNew)
			n.Send(selSetWantsLayer, true)
			v := &view{native: n, content: n, surface: s, kind: driver.KindView, up: parent,
				frame: f, framed: true, style: driver.ViewStyle{Opacity: 1, Transform: tr}}
			views[n] = v
			n.Send(selSetFrame, apple.Rect(float64(f.Origin.X), float64(f.Origin.Y), float64(f.Size.Width), float64(f.Size.Height)))
			if parent != nil {
				parent.native.Send(selAddSubview, n)
			}
			apple.SetTransform(n.Send(selLayer), tr)
			t.Cleanup(func() {
				onMain(func() { delete(views, n); apple.ForgetTransform(n.Send(selLayer)); n.Send(selRelease) })
			})
			return v
		}
		root := makeView(nil, driver.Bounds{Size: driver.Size{Width: 400, Height: 400}}, driver.Transform{})
		parent := makeView(root, driver.Bounds{Origin: driver.Point{X: 100, Y: 100}, Size: driver.Size{Width: 40, Height: 40}}, driver.Transform{Rotate: 90, Scale: 2})
		child := makeView(parent, driver.Bounds{Origin: driver.Point{X: 5, Y: 5}, Size: driver.Size{Width: 20, Height: 20}}, driver.Transform{TranslateY: -100})
		child.events = driver.EventMask(1) << driver.EventPress
		if got := root.native.Send(sel("hitTest:"), apple.CGPoint{X: 330, Y: 110}); got != child.native {
			t.Errorf("drawn child hit = %v, want %v", got, child.native)
		}
		if got := root.native.Send(sel("hitTest:"), apple.CGPoint{X: 115, Y: 115}); got == child.native {
			t.Error("old layout position still hits child")
		}
		parent.style.Clip = true
		if got := root.native.Send(sel("hitTest:"), apple.CGPoint{X: 330, Y: 110}); got == child.native {
			t.Error("clipped transformed child still takes pointer")
		}
	})
}
