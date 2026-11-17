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
	state := func(src driver.ImageSource) (int, bool) {
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
