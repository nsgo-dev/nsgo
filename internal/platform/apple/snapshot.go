//go:build darwin

package apple

import (
	"math"

	"github.com/ebitengine/purego/objc"
)

// LayerImage draws layer's tree, w by h points from its origin, into a new
// sRGB CGImage at scale pixels a point, which the caller releases; 0 when
// it cannot. Call it on the main thread. renderInContext: draws no Metal
// layer, video, web page, backdrop blur or glass, as Picture does not.
func LayerImage(layer objc.ID, w, h, scale float64) uintptr {
	pw, ph := int(math.Ceil(w*scale)), int(math.Ceil(h*scale))
	if layer == 0 || pw <= 0 || ph <= 0 {
		return 0
	}
	bitmapsLoaded.Do(loadBitmaps)
	const premultipliedFirst, byteOrder32Little = 2, 2 << 12
	ctx := cgBitmapContextCreate(0, uintptr(pw), uintptr(ph), 8, uintptr(pw*4), srgb, premultipliedFirst|byteOrder32Little)
	if ctx == 0 {
		return 0
	}
	defer Release(ctx)
	cgContextClearRect(ctx, Rect(0, 0, float64(pw), float64(ph)))
	// Layers draw with y down from the top, as the bitmap's rows go.
	cgContextTranslateCTM(ctx, 0, float64(ph))
	cgContextScaleCTM(ctx, scale, -scale)
	WithoutAnimation(func() { layer.Send(sel("renderInContext:"), ctx) })
	return cgBitmapContextCreateImage(ctx)
}

// DrawImage draws image into ctx's rectangle r.
func DrawImage(ctx uintptr, r CGRect, image uintptr) { cgContextDrawImage(ctx, r, image) }
