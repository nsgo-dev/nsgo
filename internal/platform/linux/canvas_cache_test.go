//go:build linux && !android

package linux

import (
	"image"
	"testing"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

func TestCanvasImageCacheBoundAndReload(t *testing.T) {
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	w := &window{animator: driver.NewAnimator(nil), env: driver.Environment{PixelRatio: 1}}
	defer w.closeCanvasImages()
	sources := canvastest.CacheSources(1000)
	load := func(src driver.ImageSource) *imageSlot {
		t.Helper()
		s := w.canvasImage(src, &view{deleted: true})
		for deadline := time.Now().Add(5 * time.Second); s.load != nil; {
			w.runTasks()
			if time.Now().After(deadline) {
				t.Fatal("image load timed out")
			}
			time.Sleep(time.Millisecond)
		}
		if s.pic == nil || s.surfaceOf(0) == 0 {
			t.Fatal("no decoded image surface")
		}
		return s
	}
	first := load(sources[0])
	old := load(sources[1])
	for i := 2; i < len(sources); i++ {
		load(sources[i])
		if i%32 == 0 {
			w.canvasImage(sources[0], &view{deleted: true})
		}
		if len(w.canvasImages) > maxCanvasImages {
			t.Fatalf("%d cached images", len(w.canvasImages))
		}
	}
	if w.canvasImages[sources[0].Key()] != first {
		t.Fatal("recently drawn image evicted")
	}
	if old.pic != nil || old.surface != 0 || old.load != nil {
		t.Fatal("evicted resources retained")
	}
	reloaded := load(sources[1])
	if reloaded == old {
		t.Fatal("evicted image slot reused")
	}
	d := (&driver.Drawing{}).DrawImageIn(sources[1], 0, 0, 2, 2)
	surface := cairoImageSurfaceCreate(0, 2, 2)
	defer cairoSurfaceDestroy(surface)
	cr := cairoCreate(surface)
	w.playCanvas(&view{drawing: d}, cr, rect{w: 2, h: 2})
	cairoDestroy(cr)
	cairoSurfaceFlush(surface)
	pix := unsafe.Slice((*byte)(cPointer(cairoImageSurfaceGetData(surface))), 4)
	if pix[0] != 0 || pix[1] != 0 || pix[2] != 255 || pix[3] != 255 {
		t.Fatalf("reloaded pixel BGRA %v", pix)
	}
	pixels := w.canvasImage(driver.ImageSource{Pixels: image.NewRGBA(image.Rect(0, 0, 2, 2))}, &view{deleted: true})
	if pixels.pic == nil {
		t.Fatal("pixel frame did not draw")
	}
	w.closeCanvasImages()
	if pixels.pic != nil || len(w.canvasFrames) != 0 || w.canvasFramePixels != 0 {
		t.Fatal("closed cache retained pixel frames")
	}
	if reloaded.pic != nil || reloaded.surface != 0 || len(w.canvasImages) != 0 || w.canvasImage(sources[1], &view{}) != nil {
		t.Fatal("closed cache retained resources")
	}
}

func TestCanvasImageCacheBoundsPendingAndDropsQueuedResults(t *testing.T) {
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	w := &window{}
	defer w.closeCanvasImages()
	sources := canvastest.CacheSources(1000)
	old := w.canvasImage(sources[0], &view{deleted: true})
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		w.mu.Lock()
		n := len(w.tasks)
		w.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("load did not post a result")
		}
	}
	for _, src := range sources[1:] {
		w.canvasImage(src, &view{deleted: true})
		if len(w.canvasImages) > maxCanvasImages || len(w.canvasWaiting) > maxCanvasImages {
			t.Fatal("pending cache grew beyond limit")
		}
	}
	w.runTasks()
	if old.pic != nil || old.load != nil {
		t.Fatal("queued completion resurrected evicted image")
	}
	w.closeCanvasImages()
	w.runTasks()
	if len(w.canvasImages) != 0 || len(w.canvasWaiting) != 0 {
		t.Fatal("closed cache revived")
	}
}
