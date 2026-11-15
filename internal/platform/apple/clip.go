//go:build darwin

package apple

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// NeedsClipHolder reports whether a clipped view also draws outside its box.
// Apple clips a layer's own shadow and outline when masksToBounds is set, so
// those views keep their decoration on the outer layer and clip children in a
// separate holder view.
func NeedsClipHolder(vs *driver.ViewStyle) bool {
	if vs == nil || !vs.Clip {
		return false
	}
	if vs.Outline.Visible() || vs.ShadowColor&0xff != 0 {
		return true
	}
	if vs.Effects != nil {
		for _, sh := range vs.Effects.Shadows {
			if !sh.Inset && sh.Color&0xff != 0 {
				return true
			}
		}
	}
	return false
}

// SetClipLayer applies child clipping to a holder layer. The holder is kept
// separate from the styled layer, whose own shadows and outlines must remain
// visible outside its bounds.
func SetClipLayer(layer objc.ID, vs *driver.ViewStyle) {
	if layer == 0 {
		return
	}
	if vs == nil || !vs.Clip {
		layer.Send(selSetMasksToBounds, false)
		return
	}
	b := objc.Send[CGRect](layer, selBounds)
	radii := vs.Radii(driver.Size{Width: driver.Points(b.Size.Width), Height: driver.Points(b.Size.Height)})
	r, masked := cornersOf(radii)
	setCorners(layer, r, masked, vs.ContinuousCorners, b.Size.Width, b.Size.Height)
	layer.Send(selSetMasksToBounds, true)
}
