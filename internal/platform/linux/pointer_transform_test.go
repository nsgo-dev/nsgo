//go:build linux && !android

package linux

import (
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestPointerHitsFollowCurrentTransforms(t *testing.T) {
	root := &view{style: driver.DefaultViewStyle, frame: driver.Bounds{Size: driver.Size{Width: 400, Height: 400}}}
	parent := &view{parent: root, style: driver.ViewStyle{Opacity: 1, Transform: driver.Transform{Rotate: 90, Scale: 2}},
		frame: driver.Bounds{Origin: driver.Point{X: 100, Y: 100}, Size: driver.Size{Width: 40, Height: 40}}}
	child := &view{id: 3, parent: parent, style: driver.DefaultViewStyle,
		frame: driver.Bounds{Origin: driver.Point{X: 5, Y: 5}, Size: driver.Size{Width: 20, Height: 20}}}
	root.children, parent.children = []*view{parent}, []*view{child}
	// The core flags the views whose children may be drawn transformed.
	root.transformedKids, parent.transformedKids = true, true
	// A bound value can move independently of the engine's style.
	child.animated = &driver.AnimatedValues{Set: 1 << driver.PropTranslateY, TranslateY: -100}
	at := driver.Point{X: 330, Y: 110}
	match := func(v *view) bool { return v == child }
	if got := hit(root, at, match); got != child {
		t.Errorf("drawn child hit = %p, want %p", got, child)
	}
	if got := hit(root, driver.Point{X: 115, Y: 115}, match); got != nil {
		t.Error("old layout position still hits child")
	}
	p := child.surfacePoint(at)
	if math.Abs(float64(p.X-10)) > .001 || math.Abs(float64(p.Y-10)) > .001 {
		t.Errorf("local = %+v, want (10,10)", p)
	}
	under, _ := viewsAt(root, at, match, nil)
	if len(under) != 1 || under[0] != child {
		t.Errorf("hover/wheel path = %v, want child", under)
	}
	// A clip, or children never transformed, keep the search to the bounds.
	parent.transformedKids = false
	if got := hit(root, at, match); got != nil {
		t.Error("child outside its unflagged parent takes pointer")
	}
	parent.transformedKids = true
	parent.props.PointerEvents = driver.PointerEventsBoxNone
	if got := hit(root, at, match); got != child {
		t.Error("box-none parent hides transformed child")
	}
	parent.props.PointerEvents = driver.PointerEventsBoxOnly
	if got := hit(root, at, match); got != nil {
		t.Error("box-only parent exposes child")
	}
}
