package capture_test

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/videoalpha/clips"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestVideoShowsWhatIsBehindItsTransparentPixels plays HEVC with alpha, transparent around a blue square in its middle half, over a red view, and checks the composited window: the red shows through around the square.
//
// Only WindowServer's composited capture shows an AVPlayerLayer; the test host's fallback, an AppKit cacheDisplayInRect: redraw, draws none, as it draws no filter. So the window also holds TestCaptureShowsCompositedEffects' inverted red box, and where the capture does not show it cyan the capture is not composited and the test skips, leaving that failure to TestCaptureShowsCompositedEffects.
func TestVideoShowsWhatIsBehindItsTransparentPixels(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("checks the macOS player layer's compositing")
	}
	path := filepath.Join(t.TempDir(), "alpha.mov")
	if err := os.WriteFile(path, clips.HEVCAlpha, 0o644); err != nil {
		t.Fatal(err)
	}
	const width, height = 300, 200
	var mu sync.Mutex
	var loaded string
	report := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		loaded = s
	}
	got := func() string {
		mu.Lock()
		defer mu.Unlock()
		return loaded
	}
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Background: core.RGB(0xff0000), Padding: core.All(20), Gap: 40, Direction: core.Row}, Children: []core.Node{
			core.Video{Source: path, Muted: true, Repeat: true, Style: core.Style{Width: core.Pt(160), Height: core.Pt(160)},
				OnLoad:  func(core.VideoInfo) { report("load") },
				OnError: func(err error) { report(err.Error()) }},
			core.View{Style: core.Style{Width: core.Pt(60), Height: core.Pt(60), Background: core.RGB(0xff0000), Filters: []core.Filter{core.Invert(1)}}},
		}}
	}, nsgotest.Size(width, height))
	app.WaitUntil(10*time.Second, func() bool { return got() != "" })
	if s := got(); s != "load" {
		t.Fatalf("the video reported %q", s)
	}
	// The colours are the display's, Display P3 on most Macs, so each check leaves room for the conversion.
	red := func(r, g, b uint32) bool { return r > 200 && g < 80 && b < 80 }
	blue := func(r, g, b uint32) bool { return b > 200 && r < 80 && g < 80 }
	cyan := func(r, g, b uint32) bool { return r <= 160 && g >= 200 && b >= 200 }
	var last string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		img := app.Capture()
		at := func(x, y core.Points) (r, g, b uint32) {
			p := image.Pt(img.Bounds().Min.X+int(float64(x)*float64(img.Bounds().Dx())/width), img.Bounds().Min.Y+int(float64(y)*float64(img.Bounds().Dy())/height))
			r, g, b, _ = img.At(p.X, p.Y).RGBA()
			return r >> 8, g >> 8, b >> 8
		}
		// The inverted box spans 220 to 280 points across and 20 to 80 down.
		if r, g, b := at(250, 50); !cyan(r, g, b) {
			t.Skipf("the capture is not WindowServer's composite (the inverted red box shows %d,%d,%d, not cyan), so it cannot show the player layer; TestCaptureShowsCompositedEffects fails for that", r, g, b)
		}
		// The video spans 20 to 180 points, its square 60 to 140.
		r1, g1, b1 := at(30, 30)
		r2, g2, b2 := at(170, 170)
		r3, g3, b3 := at(100, 100)
		if red(r1, g1, b1) && red(r2, g2, b2) && blue(r3, g3, b3) {
			return
		}
		last = fmt.Sprintf("%d,%d,%d and %d,%d,%d at the video's corners and %d,%d,%d in its middle", r1, g1, b1, r2, g2, b2, r3, g3, b3)
	}
	t.Errorf("want red around a blue square, have %s", last)
}
