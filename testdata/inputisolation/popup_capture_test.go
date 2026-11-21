//go:build darwin && !ios

package main

import (
	"image"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestNativePopupScreenshot(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("checks macOS native popup capture")
	}
	const width, height = 400, 280
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(width), Height: core.Pt(height), Background: core.RGB(0x9f2440)},
			Children: []core.Node{core.View{TestID: "menu-trigger", Style: core.Style{Width: core.Pt(120), Height: core.Pt(80)},
				OnContextMenu: func(e core.ContextMenuEvent) {
					desktop.PopupMenu(cx, []desktop.MenuItem{{Label: "Rename"}, {Label: "Delete"}}, e.PagePosition, nil)
				}}},
		}
	}, nsgotest.Size(width, height))
	before := app.Capture()
	cover := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		desktop.SetWindowLevel(cx, desktop.WindowLevelFloating)
		return core.View{Style: core.Style{Width: core.Pt(width), Height: core.Pt(height), Background: core.RGB(0x2ae369)}}
	}, nsgotest.Size(width, height))
	coverImage := cover.Capture()
	app.RightClick("id:menu-trigger")
	opened := app.Capture()
	app.ClosePopup()
	closed := app.Capture()
	for _, img := range []image.Image{opened, closed, coverImage} {
		if img.Bounds() != before.Bounds() {
			t.Fatalf("capture size changed: got %v, want %v", img.Bounds(), before.Bounds())
		}
	}
	pixel := func(img image.Image, x, y int) [4]uint32 {
		b := img.Bounds()
		r, g, bl, a := img.At(b.Min.X+b.Dx()*x/width, b.Min.Y+b.Dy()*y/height).RGBA()
		return [4]uint32{r >> 8, g >> 8, bl >> 8, a >> 8}
	}
	background := pixel(before, 300, 200)
	coverColor := pixel(coverImage, 300, 200)
	if background == coverColor {
		t.Fatal("the two windows do not have distinct backgrounds")
	}
	changed := 0
	for y := 20; y < height-20; y += 4 {
		for x := 20; x < width-20; x += 4 {
			baseline := pixel(before, x, y)
			if c := pixel(opened, x, y); c == coverColor {
				t.Fatalf("the popup capture shows the covering window at (%d,%d)", x, y)
			} else if c != baseline {
				changed++
			}
			if got := pixel(closed, x, y); got != baseline {
				t.Fatalf("closing the popup left pixel (%d,%d) at %v, want %v", x, y, got, baseline)
			}
		}
	}
	if changed < 50 {
		t.Fatalf("only %d samples changed when the native menu opened", changed)
	}
}
