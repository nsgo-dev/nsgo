//go:build darwin

package apple

import (
	"testing"

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
