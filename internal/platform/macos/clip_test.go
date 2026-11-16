//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestAHolderIsHitAsItsView hits a view whose children sit in a holder (a clip that keeps the view's shadow, or an effect) as the view itself where no child is: a box-none view lets the pointer through, a pressable one takes it, and a child still takes its own.
func TestAHolderIsHitAsItsView(t *testing.T) { onMain(func() { aHolderIsHitAsItsView(t) }) }

func aHolderIsHitAsItsView(t *testing.T) {
	loadAppKit()
	up := objc.ID(classView).Send(selNew)
	defer up.Send(selRelease)
	up.Send(selSetFrame, apple.Rect(0, 0, 300, 300))
	native := objc.ID(classView).Send(selNew)
	defer native.Send(selRelease)
	native.Send(selSetFrame, apple.Rect(50, 50, 200, 200))
	up.Send(selAddSubview, native)
	s := &surface{}
	v := &view{kind: driver.KindView, native: native, content: native, surface: s}
	views[native] = v
	defer delete(views, native)
	childNative := objc.ID(classView).Send(selNew)
	defer childNative.Send(selRelease)
	childNative.Send(selSetFrame, apple.Rect(0, 0, 40, 40))
	native.Send(selAddSubview, childNative)
	child := &view{kind: driver.KindView, native: childNative, content: childNative, surface: s, up: v}
	views[childNative] = child
	defer delete(views, childNative)

	holder := s.container(v)
	defer delete(views, holder)
	if holder == native || childNative.Send(selSuperview) != holder {
		t.Fatal("the child did not move into the holder")
	}
	// Asked of the view, at points in its superview's coordinates: up, out of a window, would take them in the window's, unflipped.
	hit := func(x, y float64) objc.ID { return native.Send(sel("hitTest:"), apple.CGPoint{X: x, Y: y}) }

	if got := hit(150, 150); got != native {
		t.Errorf("the view's empty area hit %v, want the view %v, not its holder %v", got, native, holder)
	}
	if got := hit(60, 60); got != childNative {
		t.Errorf("the child hit %v, want %v", got, childNative)
	}
	v.props.PointerEvents = driver.PointerEventsBoxNone
	if got := hit(150, 150); got != 0 {
		t.Errorf("a box-none view's empty area hit %v; the pointer should go through it", got)
	}
	if got := hit(60, 60); got != childNative {
		t.Errorf("a box-none view's child hit %v, want %v", got, childNative)
	}
}
