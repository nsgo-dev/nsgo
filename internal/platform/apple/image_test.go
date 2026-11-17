//go:build darwin

package apple

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestImageSizeReadsTheHeader(t *testing.T) {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 7, 3)))
	if got, err := imageSize(b.Bytes()); err != nil || got != (driver.Size{Width: 7, Height: 3}) {
		t.Fatalf("PNG size = %v, %v", got, err)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"/>`)
	if got, err := imageSize(svg); err != nil || got != (driver.Size{Width: 20, Height: 10}) {
		t.Fatalf("SVG size = %v, %v", got, err)
	}
	if _, err := imageSize([]byte("not an image")); err == nil {
		t.Fatal("no error for bytes that are no image")
	}
}

func TestDecodePointsPerPixel(t *testing.T) {
	view := CGSize{Width: 100, Height: 100}
	for _, c := range []struct {
		mode driver.ResizeMode
		w, h float64
		want float64
	}{
		{driver.Center, 400, 200, 0.5}, // a 2x file at its natural size
		{driver.ScaleDown, 400, 200, 0.25},
		{driver.ScaleDown, 100, 50, 0.5},
		{driver.Contain, 400, 200, 0.25},
		{driver.Cover, 400, 200, 0.5},
	} {
		r := request{mode: c.mode, view: view, scale: 2}
		if got := r.pointsPerPixel(c.w, c.h); got != c.want {
			t.Errorf("%v %gx%g: %g points per pixel, want %g", c.mode, c.w, c.h, got, c.want)
		}
	}
}

// An image view shows pixels at once and swaps a new version in place, reporting no loads.
func TestImageViewShowsPixelsInPlace(t *testing.T) {
	host := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	defer host.Send(selRelease)
	iv := NewImageView(host)
	defer iv.Close()
	var events []driver.EventKind
	iv.Dispatch = func(ev driver.Event) { events = append(events, ev.Kind) }
	contents := func() uintptr { return objc.Send[uintptr](iv.picture, sel("contents")) }
	frame := image.NewRGBA(image.Rect(0, 0, 40, 20))
	iv.Configure(&driver.ImageConfig{Source: driver.ImageSource{Pixels: frame, Version: 1}, ResizeMode: driver.Contain})
	iv.Layout(CGSize{Width: 80, Height: 40})
	iv.Start(2)
	first := contents()
	if first == 0 || cgImageGetWidth(first) != 40 || cgImageGetHeight(first) != 20 {
		t.Fatalf("contents = %#x, want a 40 by 20 image at once", first)
	}
	if fit := iv.fitted(); fit.Size.Width != 80 || fit.Size.Height != 40 {
		t.Fatalf("fitted = %+v, want the view filled by a point a pixel", fit)
	}
	iv.Configure(&driver.ImageConfig{Source: driver.ImageSource{Pixels: frame, Version: 2}, ResizeMode: driver.Contain})
	iv.Start(2)
	if next := contents(); next == 0 || next == first {
		t.Fatalf("version 2 shows %#x, want a new image in place of %#x", next, first)
	}
	if len(events) != 0 {
		t.Fatalf("events = %v, want none for pixels", events)
	}
}

func TestCanvasFramesKeepTheLatest(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 6, 3))
	src := driver.ImageSource{Pixels: frame}
	im := canvasImage(src, CanvasHost{})
	if im == nil || im.image == 0 || im.points != (driver.Size{Width: 6, Height: 3}) {
		t.Fatalf("image = %+v, want 6 by 3 points at once", im)
	}
	if again := canvasImage(src, CanvasHost{}); again != im {
		t.Fatal("the same copy made a new image")
	}
	for range maxCanvasFrames + 2 {
		canvasImage(driver.ImageSource{Pixels: image.NewRGBA(image.Rect(0, 0, 1, 1))}, CanvasHost{})
	}
	if len(canvasFrames) != maxCanvasFrames {
		t.Fatalf("kept %d frames, want %d", len(canvasFrames), maxCanvasFrames)
	}
	// Frames over half the pixel budget each keep only the latest.
	for range 3 {
		canvasImage(driver.ImageSource{Pixels: image.NewRGBA(image.Rect(0, 0, 2048, 2049))}, CanvasHost{})
	}
	if len(canvasFrames) != 1 || canvasFramePixels != 2048*2049 {
		t.Fatalf("kept %d frames of %d pixels, want the latest alone", len(canvasFrames), canvasFramePixels)
	}
}
