//go:build android && nsgotest

package android

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload/imageloadtest"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// Uses the real pipe, per-surface cache, decoder and CanvasNode on Android.
func TestCanvasImageNativeCacheBoundAndReload(t *testing.T) {
	sources := canvastest.CacheSources(1000)
	blue := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			blue.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, blue); err != nil {
		t.Fatal(err)
	}
	sources[999].Data = encoded.Bytes()
	var scope *core.Scope
	var choose func(int)
	surface, err := testhost.Current().Open(func(cx *core.Scope) core.Node {
		scope = cx
		selected := core.NewSignal(cx, 0)
		choose = selected.Set
		return core.Canvas{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}, Draw: func(d *core.Drawing, size core.Size) {
			s := sources[selected.Get()]
			d.DrawImageIn(core.ImageSource{Data: s.Data}, 0, 0, float32(size.Width), float32(size.Height))
		}}
	}, testhost.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := surface.(*testSurface)
	s.Sync()
	state := func(src driver.ImageSource) (int, bool) { return canvasImageState(t, s, src) }
	checkPixels := func(red bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			data, err := s.Screenshot()
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			c := color.NRGBAModel.Convert(img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2)).(color.NRGBA)
			if red && c.R == 255 && c.G == 0 && c.B == 0 || !red && c.R == 0 && c.G == 0 && c.B == 255 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("reloaded image pixel %v, red=%v", c, red)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	for i, src := range sources {
		if i != 0 {
			scope.Post(func() { choose(i) })
			s.Sync()
		}
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
			count, ready := state(src)
			if count > maxCanvasImages {
				t.Fatalf("source %d: %d cached images", i, count)
			}
			if ready {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("source %d did not decode", i)
			}
		}
		if i == 0 {
			checkPixels(true)
		}
	}
	checkPixels(false)
	if _, ready := state(sources[0]); ready {
		t.Fatal("oldest decoded image was not evicted")
	}
	scope.Post(func() { choose(0) })
	s.Sync()
	checkPixels(true)
}

// A drawing of more sources than the bound keeps them all and draws them all, rather than evicting some at every redraw and loading them again forever.
func TestCanvasImageNativeCacheKeepsAWideDrawing(t *testing.T) {
	sources := canvastest.CacheSources(2 * maxCanvasImages)
	surface, err := testhost.Current().Open(func(cx *core.Scope) core.Node {
		return core.Canvas{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}, Draw: func(d *core.Drawing, size core.Size) {
			for i, s := range sources {
				d.DrawImageIn(core.ImageSource{Data: s.Data}, float32(i%16)*4, float32(i/16)*4, 4, 4)
			}
		}}
	}, testhost.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := surface.(*testSurface)
	s.Sync()
	state := func(src driver.ImageSource) (int, bool) { return canvasImageState(t, s, src) }
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		ready := 0
		count := 0
		for _, src := range sources {
			n, ok := state(src)
			count = n
			if ok {
				ready++
			}
		}
		if ready == len(sources) {
			if count != len(sources) {
				t.Fatalf("%d cached for a drawing of %d", count, len(sources))
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d sources drawn, %d cached", ready, len(sources), count)
		}
	}
}

// canvasImageState returns how many entries s's canvas cache holds and whether src's has decoded, without making it recently drawn.
func canvasImageState(t *testing.T, s *testSurface, src driver.ImageSource) (int, bool) {
	t.Helper()
	data, err := s.ask(map[string]any{"op": "canvas-images", "key": src.Key()})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Count int
		Ready bool
	}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatal(err)
	}
	return result.Count, result.Ready
}

// Drawings that follow each other with more slow images together than the bound keep every load under way, past the bound, rather than cancel the earlier ones' loads, and every image arrives; drawing others then shrinks the cache back to the bound.
func TestCanvasImageNativeCacheKeepsLoadsUnderWay(t *testing.T) {
	var sets [][]driver.ImageSource
	var scope *core.Scope
	var choose func(int)
	surface, err := testhost.Current().Open(func(cx *core.Scope) core.Node {
		scope = cx
		selected := core.NewSignal(cx, -1)
		choose = selected.Set
		return core.Canvas{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}, Draw: func(d *core.Drawing, size core.Size) {
			if i := selected.Get(); i >= 0 {
				for j, src := range sets[i] {
					d.DrawImageIn(src, float32(j%16)*4, float32(j/16)*4, 4, 4)
				}
			}
		}}
	}, testhost.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := surface.(*testSurface)
	s.Sync()
	// The surface set imageload.HTTP as it started: Slow replaces it from here on.
	var slow []driver.ImageSource
	for _, uri := range imageloadtest.Slow(t, 88, 2*time.Second, canvastest.RedPNG()) {
		slow = append(slow, driver.ImageSource{URI: uri})
	}
	sets = [][]driver.ImageSource{slow[:40], slow[40:80], slow[80:], canvastest.CacheSources(maxCanvasImages)}
	show := func(i, want int) {
		t.Helper()
		scope.Post(func() { choose(i) })
		s.Sync()
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
			count, _ := canvasImageState(t, s, sets[i][0])
			if count == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("drawing set %d: %d cached, want %d", i, count, want)
			}
		}
	}
	// Each set draws in a pass of its own while the earlier sets' images still load.
	show(0, 40)
	show(1, 80)
	show(2, 88)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		ready := 0
		for _, src := range slow {
			if _, ok := canvasImageState(t, s, src); ok {
				ready++
			}
		}
		if ready == len(slow) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d slow images arrived", ready, len(slow))
		}
	}
	show(3, maxCanvasImages)
}
