//go:build darwin

package apple

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

func isolatedCanvasImages(t *testing.T) {
	t.Helper()
	saved := canvasImages
	canvasImages = map[string]*loadedImage{}
	t.Cleanup(func() {
		for key, im := range canvasImages {
			if im.cancel != nil {
				im.cancel()
			}
			Release(im.image)
			im.image = 0
			delete(canvasImages, key)
		}
		canvasImages = saved
		runMain()
	})
}

func TestCanvasImageCacheBoundAndReload(t *testing.T) {
	isolatedCanvasImages(t)
	sources := canvastest.CacheSources(1000)
	load := func(src driver.ImageSource) *loadedImage {
		t.Helper()
		canvasImage(src, CanvasHost{Ratio: 1})
		im := canvasImages[src.Key()]
		for deadline := time.Now().Add(5 * time.Second); im.cancel != nil; {
			runMain()
			if time.Now().After(deadline) {
				t.Fatal("image load timed out")
			}
			time.Sleep(time.Millisecond)
		}
		if im.image == 0 {
			t.Fatal("no decoded image")
		}
		return im
	}
	first := load(sources[0])
	old := load(sources[1])
	for i := 2; i < len(sources); i++ {
		load(sources[i])
		if i%32 == 0 {
			canvasImage(sources[0], CanvasHost{})
		}
		if len(canvasImages) > maxCanvasImages {
			t.Fatalf("%d cached images", len(canvasImages))
		}
	}
	if canvasImages[sources[0].Key()] != first {
		t.Fatal("recently drawn image evicted")
	}
	if old.image != 0 {
		t.Fatal("evicted native image retained")
	}
	reloaded := load(sources[1])
	if reloaded == old {
		t.Fatal("evicted image slot reused")
	}
	d := (&driver.Drawing{}).DrawImageIn(sources[1], 0, 0, 2, 2)
	img := play(t, "cache-reload", d, 2, 2, false)
	if got := img.RGBAAt(1, 1); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("reloaded pixel %v", got)
	}
}

func TestCanvasImageCacheBoundsPendingAndFailed(t *testing.T) {
	isolatedCanvasImages(t)
	sources := canvastest.CacheSources(1000)
	canvasImage(sources[0], CanvasHost{})
	old := canvasImages[sources[0].Key()]
	for _, src := range sources[1:] {
		canvasImage(src, CanvasHost{})
		if len(canvasImages) > maxCanvasImages {
			t.Fatal("pending cache grew beyond limit")
		}
	}
	runMain()
	if old.image != 0 || old.waiting != nil {
		t.Fatal("obsolete completion revived a cache entry")
	}
	// Failed loads must be evictable too, even with no decoded native image.
	for key, im := range canvasImages {
		if im.cancel != nil {
			im.cancel()
		}
		Release(im.image)
		delete(canvasImages, key)
	}
	for i := range maxCanvasImages {
		canvasImages[fmt.Sprint(i)] = &loadedImage{failed: true, used: uint64(i)}
	}
	canvasImage(sources[0], CanvasHost{})
	if len(canvasImages) != maxCanvasImages {
		t.Fatal("failed entries prevented eviction")
	}
	for deadline := time.Now().Add(5 * time.Second); canvasImages[sources[0].Key()].cancel != nil; {
		runMain()
		if time.Now().After(deadline) {
			t.Fatal("reload timed out")
		}
		time.Sleep(time.Millisecond)
	}
}
