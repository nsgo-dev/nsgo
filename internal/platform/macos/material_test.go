//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestMaterialCode(t *testing.T) {
	cases := map[driver.WindowMaterial]int{
		driver.WindowMaterialWindow:       12,
		driver.WindowMaterialContent:      18,
		driver.WindowMaterialSidebar:      7,
		driver.WindowMaterialHeader:       10,
		driver.WindowMaterialTitlebar:     3,
		driver.WindowMaterialUnderWindow:  21,
		driver.WindowMaterialUnderPage:    22,
		driver.WindowMaterialMenu:         5,
		driver.WindowMaterialPopover:      6,
		driver.WindowMaterialHUD:          13,
		driver.WindowMaterialSheet:        11,
		driver.WindowMaterialTooltip:      17,
		driver.WindowMaterialSelection:    4,
		driver.WindowMaterialFullScreenUI: 15,
		driver.WindowMaterialMica:         21,
		driver.WindowMaterialAcrylic:      13,
		driver.WindowMaterialTabbed:       21,
	}
	for m, want := range cases {
		if got, ok := materialCode(m); !ok || got != want {
			t.Errorf("materialCode(%d) = %d, %v; want %d, true", m, got, ok, want)
		}
	}
	for _, m := range []driver.WindowMaterial{driver.WindowMaterialNone, driver.WindowMaterial(100)} {
		if code, ok := materialCode(m); ok {
			t.Errorf("materialCode(%d) = %d, true; want no material", m, code)
		}
	}
}

func TestViewsClipAsTheirStyleSays(t *testing.T) {
	if clipsBounds(nil, 1) {
		t.Fatal("a view not yet registered clips, masking its layer for good")
	}
	plain := &view{native: 1, content: 1}
	if clipsBounds(plain, 1) {
		t.Fatal("a view whose style does not clip clips")
	}
	plain.style.Clip = true
	if !clipsBounds(plain, 1) {
		t.Fatal("a view whose style clips does not")
	}
	// A holder clips the children of a view with a shadow, whose own layer would clip the shadow.
	plain.content, plain.clipHeld = 2, true
	if clipsBounds(plain, 1) {
		t.Fatal("a view whose holder clips its children clips its own shadow")
	}
	if !clipsBounds(plain, 2) {
		t.Fatal("the holder of a view that clips does not")
	}
}

// TestApplyAfterClose ignores the apply message of a surface closed while
// it waited on the main thread.
func TestApplyAfterClose(t *testing.T) {
	surfaceApply(0, 0, 0)
}

// A view clips what it holds without its own layer only where that layer would clip its shadow or outline with it, and only where a holder or its image layer clips all it holds.
func TestViewsClipInAHolderToKeepTheirShadow(t *testing.T) {
	shadow := &driver.Effects{Shadows: []driver.BoxShadow{{Blur: 24, Color: driver.RGBA(0, 0x59)}}}
	card := driver.ViewStyle{Clip: true, CornerRadius: 16, Effects: shadow}
	shaped := card
	shaped.Corners = driver.Corners{TopLeft: 16, TopRight: 4, BottomRight: 4, BottomLeft: 4}
	for _, c := range []struct {
		name string
		v    *view
		vs   driver.ViewStyle
		want bool
	}{
		{"a card", &view{kind: driver.KindView}, card, true},
		{"a card of corners that differ", &view{kind: driver.KindView}, shaped, true},
		{"a card that does not clip", &view{kind: driver.KindView}, driver.ViewStyle{CornerRadius: 16, Effects: shadow}, false},
		{"a clip with no shadow", &view{kind: driver.KindView}, driver.ViewStyle{Clip: true, CornerRadius: 16}, false},
		{"a canvas", &view{kind: driver.KindCanvas}, card, false},
		{"an image, which clips to one radius itself", &view{kind: driver.KindImage}, card, true},
		{"an image of corners that differ", &view{kind: driver.KindImage}, shaped, false},
		{"a scroll view", &view{kind: driver.KindScroll, sc: &scrollState{}}, card, true},
		{"a scroll view of corners that differ", &view{kind: driver.KindScroll, sc: &scrollState{}}, shaped, false},
	} {
		if got := c.v.clipsInHolder(&c.vs); got != c.want {
			t.Errorf("%s: clipsInHolder = %v, want %v", c.name, got, c.want)
		}
	}
}
