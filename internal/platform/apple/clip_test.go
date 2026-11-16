//go:build darwin

package apple

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestNeedsClipHolder(t *testing.T) {
	plain := driver.ViewStyle{Clip: true}
	if NeedsClipHolder(&plain) {
		t.Fatal("plain clipping should stay on the styled layer")
	}
	if NeedsClipHolder(&driver.ViewStyle{ShadowColor: driver.RGB(0x112233)}) {
		t.Fatal("a shadow without clipping needs no holder")
	}
	if !NeedsClipHolder(&driver.ViewStyle{Clip: true, ShadowColor: driver.RGB(0x112233)}) {
		t.Fatal("a clipped native shadow needs a holder")
	}
	if !NeedsClipHolder(&driver.ViewStyle{Clip: true, Outline: driver.Outline{Width: 1, Color: driver.RGB(0x112233)}}) {
		t.Fatal("a clipped outline needs a holder")
	}
	if NeedsClipHolder(&driver.ViewStyle{Clip: true, Effects: &driver.Effects{Shadows: []driver.BoxShadow{{Inset: true, Color: driver.RGB(0x112233)}}}}) {
		t.Fatal("an inset shadow stays inside the clip")
	}
	if !NeedsClipHolder(&driver.ViewStyle{Clip: true, Effects: &driver.Effects{Shadows: []driver.BoxShadow{{Color: driver.RGB(0x112233)}}}}) {
		t.Fatal("an outer box shadow needs a holder")
	}
}

func TestShapedCorners(t *testing.T) {
	for _, c := range []struct {
		name string
		vs   driver.ViewStyle
		want bool
	}{
		{"square", driver.ViewStyle{}, false},
		{"one radius", driver.ViewStyle{CornerRadius: 12}, false},
		{"the top corners only", driver.ViewStyle{Corners: driver.Corners{TopLeft: 12, TopRight: 12}}, false},
		{"radii that differ", driver.ViewStyle{Corners: driver.Corners{TopLeft: 16, TopRight: 4, BottomRight: 4, BottomLeft: 4}}, true},
	} {
		if got := ShapedCorners(&c.vs); got != c.want {
			t.Errorf("%s: ShapedCorners = %v, want %v", c.name, got, c.want)
		}
	}
}

// A holder clips to the view's corners: its own corner radius for one radius, a shape mask for radii that differ, which gives way to an effect's mask and comes back with HolderMask, and nothing once the view stops clipping in it.
func TestClipHolderClipsToTheCorners(t *testing.T) {
	layer := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	defer layer.Send(selRelease)
	defer ForgetHolder(layer)
	layer.Send(sel("setBounds:"), CGRect{Size: CGSize{Width: 200, Height: 120}})
	mask := func() objc.ID { return layer.Send(sel("mask")) }
	clips := func() bool { return objc.Send[bool](layer, sel("masksToBounds")) }
	radius := func() float64 { return objc.Send[float64](layer, sel("cornerRadius")) }

	ClipHolder(layer, &driver.ViewStyle{Clip: true, CornerRadius: 16}, false)
	if !clips() || radius() != 16 || mask() != 0 {
		t.Fatalf("one radius: masksToBounds %v, cornerRadius %v, mask %v; want true, 16, none", clips(), radius(), mask())
	}

	shaped := &driver.ViewStyle{Clip: true, Corners: driver.Corners{TopLeft: 16, TopRight: 4, BottomRight: 4, BottomLeft: 4}}
	ClipHolder(layer, shaped, false)
	m := HolderMask(layer)
	if m == 0 || mask() != m || clips() || radius() != 0 {
		t.Fatalf("radii that differ: mask %v (kept %v), masksToBounds %v, cornerRadius %v; want the kept mask, false, 0", mask(), m, clips(), radius())
	}
	if f := objc.Send[CGRect](m, sel("frame")); f.Size != (CGSize{Width: 200, Height: 120}) {
		t.Fatalf("mask frame %v, want the holder's bounds", f)
	}

	// An effect hides the children with a mask of its own; laying the holder out again leaves it.
	hide := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	defer hide.Send(selRelease)
	layer.Send(selSetMask, hide)
	ClipHolder(layer, shaped, false)
	if mask() != hide {
		t.Fatal("the holder's clip replaced an effect's mask")
	}
	layer.Send(selSetMask, HolderMask(layer))
	if mask() != m {
		t.Fatal("the holder's clip did not come back")
	}

	ClipHolder(layer, nil, false)
	if mask() != 0 || HolderMask(layer) != 0 || clips() || radius() != 0 {
		t.Fatalf("unclipped: mask %v, masksToBounds %v, cornerRadius %v; want none, false, 0", mask(), clips(), radius())
	}
	ClipHolder(layer, nil, true)
	if !clips() {
		t.Fatal("a scroll view's container stopped clipping to its bounds")
	}
}
