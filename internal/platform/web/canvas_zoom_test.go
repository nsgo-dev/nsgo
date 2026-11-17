//go:build js && wasm

package web

import (
	"math"
	"syscall/js"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func canvasZoomSurface(t *testing.T) (*surface, *view) {
	t.Helper()
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("run with nsgo test -target web")
	}
	host := doc.Call("createElement", "div")
	doc.Get("body").Call("appendChild", host)
	s := &surface{doc: doc, host: host, animator: driver.NewAnimator(nil), env: driver.Environment{PixelRatio: 2, ReduceMotion: true}}
	s.parked = s.element("div")
	host.Call("appendChild", s.parked)
	t.Cleanup(func() {
		if t.Failed() {
			for _, v := range s.views {
				if v != nil && v.sc != nil {
					t.Logf("scroll %d: zoom=%v, canvases=%d, timer=%v", v.id, v.sc.zoom, v.content.Call("querySelectorAll", "canvas").Length(), v.sc.rasterTimer)
				}
				if v != nil && v.drawing != nil {
					t.Logf("canvas %d: scale=%v, last=%v", v.id, s.canvasScale(v), v.canvasRatio)
				}
			}
		}
		for _, v := range s.views {
			if v == nil {
				continue
			}
			if v.sc != nil {
				v.sc.stopRaster()
				v.sc.stopZoom()
			}
			v.listeners.release()
		}
		host.Call("remove")
	})
	f := driver.Bounds{Size: driver.Size{Width: 100, Height: 100}}
	c := driver.ScrollConfig{MinZoomScale: .5, MaxZoomScale: 8, ZoomScale: 1, HideIndicator: true}
	d := driver.NewDrawing(nil)
	d.SetFillColor(driver.RGB(0xffffff)).FillRect(0, 0, 100, 100)
	d.SetFillColor(driver.RGB(0x000000)).FillRect(10, 0, 1, 100)
	s.mount(&driver.MountBatch{Root: 1, Items: []driver.MountItem{
		{Kind: driver.ItemCreate, ID: 1, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 2, View: driver.KindScroll},
		{Kind: driver.ItemInsert, ID: 2, Parent: 1},
		{Kind: driver.ItemFrame, ID: 2, Frame: f},
		{Kind: driver.ItemScrollContent, ID: 2, Size: f.Size},
		{Kind: driver.ItemScrollConfig, ID: 2, Scroll: &c},
		{Kind: driver.ItemCreate, ID: 3, View: driver.KindScroll},
		{Kind: driver.ItemInsert, ID: 3, Parent: 1},
		{Kind: driver.ItemFrame, ID: 3, Frame: f},
		{Kind: driver.ItemScrollContent, ID: 3, Size: f.Size},
		{Kind: driver.ItemScrollConfig, ID: 3, Scroll: &c},
		{Kind: driver.ItemCreate, ID: 4, View: driver.KindView},
		{Kind: driver.ItemInsert, ID: 4, Parent: 2},
		{Kind: driver.ItemFrame, ID: 4, Frame: f},
		{Kind: driver.ItemCreate, ID: 5, View: driver.KindCanvas},
		{Kind: driver.ItemInsert, ID: 5, Parent: 4},
		{Kind: driver.ItemFrame, ID: 5, Frame: f},
		{Kind: driver.ItemCanvas, ID: 5, Drawing: d},
	}})
	return s, s.view(5)
}

func waitCanvasPixels(t *testing.T, v *view, size int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if v.inner.Get("width").Int() == size && v.inner.Get("height").Int() == size {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("canvas bitmap is %dx%d, want %dx%d", v.inner.Get("width").Int(), v.inner.Get("height").Int(), size, size)
		}
	}
}

func TestCanvasScrollZoomFollowsKeptPlacement(t *testing.T) {
	s, canvas := canvasZoomSurface(t)
	waitCanvasPixels(t, canvas, 200)
	s.zoomCommand(s.view(2), &driver.ZoomRequest{Scale: 3}, false)
	waitCanvasPixels(t, canvas, 600)
	s.zoomCommand(s.view(3), &driver.ZoomRequest{Scale: 2}, false)
	// Only the holder moves; its canvas sends no frame or drawing item.
	s.mount(&driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemInsert, ID: 4, Parent: 3}}})
	waitCanvasPixels(t, canvas, 400)
	s.mount(&driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemRemove, ID: 4}, {Kind: driver.ItemInsert, ID: 4, Parent: 1}}})
	waitCanvasPixels(t, canvas, 200)
	// A later screen-scale change still takes the current placement.
	s.env.PixelRatio = 3
	s.paint(canvas)
	waitCanvasPixels(t, canvas, 300)
}

func TestCanvasScrollZoomFollowsWheelAndConfiguration(t *testing.T) {
	s, canvas := canvasZoomSurface(t)
	scroll := s.view(2)
	e := js.Global().Get("WheelEvent").New("wheel", map[string]any{
		"ctrlKey": true, "deltaY": -100 * math.Log(3), "clientX": 0, "clientY": 0, "cancelable": true,
	})
	s.scrollWheelZoom(scroll, e)
	waitCanvasPixels(t, canvas, 600)
	// Configuration can clamp a live zoom without a ZoomTo command.
	s.zoomCommand(scroll, &driver.ZoomRequest{Scale: 8}, true)
	time.Sleep(30 * time.Millisecond)
	c := scroll.sc.config
	c.MaxZoomScale = 2
	s.configureScroll(scroll, &c)
	waitCanvasPixels(t, canvas, 400)
	time.Sleep(300 * time.Millisecond)
	waitCanvasPixels(t, canvas, 400)
	c.MaxZoomScale = c.MinZoomScale
	s.configureScroll(scroll, &c)
	waitCanvasPixels(t, canvas, 200)
	// Raster resolution is bounded, as on Apple, even at larger zooms.
	c.MinZoomScale, c.MaxZoomScale = 1, 8
	s.configureScroll(scroll, &c)
	s.zoomCommand(scroll, &driver.ZoomRequest{Scale: 8}, false)
	waitCanvasPixels(t, canvas, 800)
	// Removing the scroll while its next raster replay waits is safe.
	s.zoomCommand(scroll, &driver.ZoomRequest{Scale: 2}, false)
	s.zoomCommand(scroll, &driver.ZoomRequest{Scale: 1}, true)
	s.mount(&driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemDelete, ID: 5}, {Kind: driver.ItemDelete, ID: 4}, {Kind: driver.ItemDelete, ID: 2},
	}})
	time.Sleep(200 * time.Millisecond)
}
