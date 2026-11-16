//go:build darwin && !ios

package macos

import (
	"slices"
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
		t.Error("the child did not move into the holder")
		return
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

// TestAHolderKeepsThePerspective keeps a view's Perspective on its children once its holder stops clipping, as the shadow it clipped for goes away, and after a resize, centred on the new size: the view's own sublayer transform reaches only the flat holder.
func TestAHolderKeepsThePerspective(t *testing.T) { onMain(func() { aHolderKeepsThePerspective(t) }) }

func aHolderKeepsThePerspective(t *testing.T) {
	loadAppKit()
	native := objc.ID(classView).Send(selNew)
	defer native.Send(selRelease)
	native.Send(selSetWantsLayer, true)
	native.Send(selSetFrame, apple.Rect(0, 0, 200, 120))
	s := &surface{}
	v := &view{kind: driver.KindView, native: native, content: native, surface: s}
	views[native] = v
	defer delete(views, native)

	// CATransform3D, by rows.
	type matrix struct{ M11, M12, M13, M14, M21, M22, M23, M24, M31, M32, M33, M34, M41, M42, M43, M44 float64 }
	sublayer := func(layer objc.ID) matrix { return objc.Send[matrix](layer, sel("sublayerTransform")) }
	// What a holder of layer's anchor and size takes for the style: the perspective centred on it.
	want := func(layer objc.ID, vs driver.ViewStyle) matrix {
		ref := objc.ID(apple.Class("CALayer")).Send(selNew)
		defer ref.Send(selRelease)
		ref.Send(sel("setAnchorPoint:"), objc.Send[apple.CGPoint](layer, sel("anchorPoint")))
		ref.Send(sel("setBounds:"), objc.Send[apple.CGRect](layer, selBounds))
		apple.ClipHolder(ref, &vs, false)
		return sublayer(ref)
	}

	shadow := &driver.Effects{Shadows: []driver.BoxShadow{{Blur: 24, Color: driver.RGBA(0, 0x59)}}}
	card := driver.ViewStyle{Clip: true, CornerRadius: 32, ChildPerspective: 300, Effects: shadow}
	if !v.holdClip(&card) {
		t.Error("a clipped card with a shadow did not clip in a holder")
		return
	}
	holder := v.content
	defer delete(views, holder)
	defer apple.ForgetHolder(holder.Send(selLayer))
	layer := holder.Send(selLayer)
	if got := sublayer(layer); got.M34 != -1.0/300 {
		t.Errorf("the held children's perspective m34 = %v, want %v", got.M34, -1.0/300)
		return
	}

	plain := card
	plain.Effects = nil
	if v.holdClip(&plain) {
		t.Error("a card without a shadow still clips in its holder")
		return
	}
	if got := sublayer(layer); got.M34 != -1.0/300 {
		t.Errorf("once the shadow went, the children's perspective m34 = %v, want %v", got.M34, -1.0/300)
		return
	}
	if objc.Send[bool](layer, sel("masksToBounds")) {
		t.Error("the holder still clips once the view's own layer does")
	}

	plain.Clip = false
	v.drawn, v.styled = plain, true
	v.place(driver.Bounds{Size: driver.Size{Width: 400, Height: 300}})
	if got, w := sublayer(layer), want(layer, plain); got != w {
		t.Errorf("after a resize the holder's sublayer transform is %v, want %v", got, w)
		return
	}
}

// TestAHolderIsNoTabStop keeps a pressable view whose children sit in a holder (a clip that keeps its shadow, or an effect) one stop in the window's key view loop: the holder answers for the view in hits, not for the focus.
func TestAHolderIsNoTabStop(t *testing.T) { onMain(func() { aHolderIsNoTabStop(t) }) }

func aHolderIsNoTabStop(t *testing.T) {
	loadAppKit()
	win := objc.ID(apple.Class("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), apple.Rect(0, 0, 400, 300), uint(0), uint(2), false)
	win.Send(sel("setReleasedWhenClosed:"), false)
	defer win.Send(selRelease)
	root := win.Send(sel("contentView"))
	s := &surface{window: win}
	var made []objc.ID
	defer func() {
		for _, n := range made {
			delete(views, n)
		}
	}()
	card := func(x float64) *view {
		n := objc.ID(classView).Send(selNew)
		n.Send(selSetFrame, apple.Rect(x, 10, 80, 80))
		root.Send(selAddSubview, n)
		n.Send(selRelease)
		v := &view{kind: driver.KindView, native: n, content: n, surface: s, events: driver.EventMask(1) << driver.EventPress}
		child := objc.ID(classView).Send(selNew)
		child.Send(selSetFrame, apple.Rect(0, 0, 20, 20))
		n.Send(selAddSubview, child)
		child.Send(selRelease)
		views[n], views[child] = v, &view{kind: driver.KindView, native: child, content: child, surface: s, up: v}
		made = append(made, n, child)
		return v
	}
	a, b, c := card(10), card(110), card(210)
	holder := s.container(b)
	made = append(made, holder)
	if objc.Send[bool](holder, sel("canBecomeKeyView")) || objc.Send[bool](holder, sel("acceptsFirstResponder")) {
		t.Error("the holder of a pressable view's children takes the focus")
	}
	win.Send(sel("recalculateKeyViewLoop"))
	names := map[objc.ID]string{a.native: "a", b.native: "b", c.native: "c", holder: "b's holder"}
	var got []string
	for at, i := a.native, 0; at != 0 && i < 4; i++ {
		got = append(got, names[at])
		at = at.Send(sel("nextValidKeyView"))
	}
	if want := []string{"a", "b", "c", "a"}; !slices.Equal(got, want) {
		t.Errorf("the key view loop from a is %q, want %q", got, want)
	}
}
