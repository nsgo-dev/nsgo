//go:build darwin

package apple

import "testing"

func TestDrawImageFillingFillsTheContext(t *testing.T) {
	bitmapsLoaded.Do(loadBitmaps)
	const premultipliedFirst, byteOrder32Little = 2, 2 << 12
	bitmap := func(side int) uintptr {
		ctx := cgBitmapContextCreate(0, uintptr(side), uintptr(side), 8, uintptr(side*4), srgb, premultipliedFirst|byteOrder32Little)
		if ctx == 0 {
			t.Fatal("no bitmap context")
		}
		return ctx
	}
	src := bitmap(4)
	red := cgColorCreateSRGB(1, 0, 0, 1)
	cgContextSetFillColor(src, red)
	Release(red)
	cgContextFillRect(src, Rect(0, 0, 4, 4))
	image := cgBitmapContextCreateImage(src)
	Release(src)
	defer Release(image)
	// A context in pixels, as QuickLook hands a reply, and one in points
	// of 2 pixels each.
	for _, scale := range []float64{1, 2} {
		ctx := bitmap(16)
		cgContextScaleCTM(ctx, scale, scale)
		DrawImageFilling(ctx, image)
		out := cgBitmapContextCreateImage(ctx)
		Release(ctx)
		img := readCGImageRGBA(out)
		Release(out)
		for _, p := range [][2]int{{0, 0}, {15, 0}, {0, 15}, {15, 15}, {8, 8}} {
			if c := img.RGBAAt(p[0], p[1]); c.R < 0xf0 || c.A < 0xf0 {
				t.Errorf("scale %v: pixel %v = %v, want the image's red", scale, p, c)
			}
		}
	}
}
