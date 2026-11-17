//go:build linux && !android

package linux

// Where a window takes the pointer (driver.WindowPointerEvents), by its surface's input region, as NSH does: an empty one lets every press, wheel turn and move through to the window below, on X11 and Wayland alike, and one of the painted views' bounds lets it through elsewhere. GTK sets the region itself as it allocates the window, for its client-side shadows, so the window sets its own again as it draws each frame that followed an allocation or changed what the views cover; giving the pointer back, it hands the region to GTK again.

import (
	"math"
	"slices"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	gdkSurfaceSetInputRegion  func(surface, region uintptr)
	gtkWidgetQueueResize      func(widget uintptr)
	cairoRegionCreate         func() uintptr
	cairoRegionUnionRectangle func(region uintptr, r *cairoRectInt) int32
	cairoRegionDestroy        func(region uintptr)
)

// cairoRectInt is cairo_rectangle_int_t.
type cairoRectInt struct{ x, y, width, height int32 }

// windowInput is what a window's surface was told of its input region.
type windowInput struct {
	mode driver.WindowPointerEvents
	// set says the window set the region, rects to what; stale that GTK may have set its own since, or the mode changed.
	set   bool
	stale bool
	rects []cairoRectInt
}

// bindWindowPointer registers the input region's bindings; bind calls it.
func bindWindowPointer() {
	purego.RegisterLibFunc(&gdkSurfaceSetInputRegion, gtkLib, "gdk_surface_set_input_region")
	purego.RegisterLibFunc(&gtkWidgetQueueResize, gtkLib, "gtk_widget_queue_resize")
	purego.RegisterLibFunc(&cairoRegionCreate, cairoLib, "cairo_region_create")
	purego.RegisterLibFunc(&cairoRegionUnionRectangle, cairoLib, "cairo_region_union_rectangle")
	purego.RegisterLibFunc(&cairoRegionDestroy, cairoLib, "cairo_region_destroy")
}

// setPointerEvents makes the window take the pointer as mode says, from its next frame.
func (w *window) setPointerEvents(mode driver.WindowPointerEvents) {
	if w.win == 0 {
		return
	}
	w.input.mode, w.input.stale = mode, true
	if mode == driver.WindowPointerAuto {
		if w.input.set {
			w.input.set, w.input.rects = false, nil
			if surface := gtkNativeGetSurface(w.win); surface != 0 {
				gdkSurfaceSetInputRegion(surface, 0)
			}
			// GTK sets its own again as it allocates the window.
			gtkWidgetQueueResize(w.win)
		}
		return
	}
	gtkWidgetQueueDraw(w.surface)
}

// updateInputRegion sets the surface's input region as the window's mode asks, as a frame draws: none for a window that takes no pointer, and the painted views' bounds for one that takes it where it draws.
func (w *window) updateInputRegion() {
	if w.win == 0 || w.input.mode == driver.WindowPointerAuto {
		return
	}
	surface := gtkNativeGetSurface(w.win)
	if surface == 0 {
		return
	}
	var rects []cairoRectInt
	if w.input.mode == driver.WindowPointerPainted {
		rects = w.paintedRects()
	}
	if w.input.set && !w.input.stale && slices.Equal(rects, w.input.rects) {
		return
	}
	region := cairoRegionCreate()
	for i := range rects {
		cairoRegionUnionRectangle(region, &rects[i])
	}
	gdkSurfaceSetInputRegion(surface, region)
	cairoRegionDestroy(region)
	w.input.set, w.input.stale, w.input.rects = true, false, rects
}

// paintedRects returns the bounds, in the surface's coordinates, of the views that take the pointer in a painted window (driver.Painted), by their pointer events, within what the views around them clip.
func (w *window) paintedRects() []cairoRectInt {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	var sx, sy float64
	gtkNativeGetSurfaceTransform(w.win, &sx, &sy)
	var out []cairoRectInt
	var walk func(v *view, clip grapheneRect, clipped bool)
	walk = func(v *view, clip grapheneRect, clipped bool) {
		pe := v.props.PointerEvents
		if v.style.Hidden || v.style.Opacity <= 0 || v.widget == 0 || pe == driver.PointerEventsNone {
			return
		}
		var b grapheneRect
		if gtkWidgetComputeBounds(v.widget, w.win, &b) == 0 {
			return
		}
		if clipped {
			b = intersectRect(b, clip)
		}
		if pe != driver.PointerEventsBoxNone && b.w > 0 && b.h > 0 && driver.Painted(v.kind, &v.style, v.events, v.props.WindowDrag) {
			x0, y0 := math.Floor(float64(b.x)+sx), math.Floor(float64(b.y)+sy)
			x1, y1 := math.Ceil(float64(b.x+b.w)+sx), math.Ceil(float64(b.y+b.h)+sy)
			out = append(out, cairoRectInt{int32(x0), int32(y0), int32(x1 - x0), int32(y1 - y0)})
		}
		if pe == driver.PointerEventsBoxOnly {
			return
		}
		if v.kind == driver.KindScroll || v.style.Clip {
			clip, clipped = b, true
		}
		for _, c := range v.children {
			walk(c, clip, clipped)
		}
	}
	walk(root, grapheneRect{}, false)
	return out
}

// intersectRect returns what a and b share, empty when nothing.
func intersectRect(a, b grapheneRect) grapheneRect {
	x0, y0 := max(a.x, b.x), max(a.y, b.y)
	x1, y1 := min(a.x+a.w, b.x+b.w), min(a.y+a.h, b.y+b.h)
	if x1 <= x0 || y1 <= y0 {
		return grapheneRect{}
	}
	return grapheneRect{x0, y0, x1 - x0, y1 - y0}
}
