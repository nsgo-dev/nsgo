//go:build darwin && !ios

package macos

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
)

// pumpMainUntil runs the main run loop, which serves the main queue
// apple.OnMain posts to, until done closes or a deadline passes.
func pumpMainUntil(t *testing.T, done <-chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			return
		default:
		}
		onMain(func() {
			until := objc.ID(apple.Class("NSDate")).Send(sel("dateWithTimeIntervalSinceNow:"), 0.02)
			objc.ID(apple.Class("NSRunLoop")).Send(sel("mainRunLoop")).Send(sel("runUntilDate:"), until)
		})
	}
	t.Fatal("the thumbnail did not finish")
}

func thumbnailFactory(got *QuickLookThumbnailRequest) quickLookThumbnailFactory {
	return func(req QuickLookThumbnailRequest) (engine.App, engine.HostOptions) {
		*got = req
		return func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{FlexGrow: 1, Background: core.RGB(0xd02030)}}
		}, engine.HostOptions{Settings: &engine.HostSettings{}}
	}
}

func TestQuickLookThumbnailDrawsTheAppOffTheScreen(t *testing.T) {
	var got QuickLookThumbnailRequest
	onMain(func() { registerQuickLookThumbnailProvider(thumbnailFactory(&got), Options{}) })
	req := QuickLookThumbnailRequest{Path: "/tmp/scene.scenra", Size: driver.Size{Width: 64, Height: 48}, Scale: 2}
	var image uintptr
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		image, err = renderQuickLookThumbnail(ctx, req)
	}()
	pumpMainUntil(t, done)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	defer apple.Release(image)
	if got != req {
		t.Errorf("the app was made for %+v, want %+v", got, req)
	}
	if w, h := apple.ImageSize(image); w != 128 || h != 96 {
		t.Fatalf("thumbnail is %dx%d pixels, want 128x96", w, h)
	}
	data, err := apple.PNGImage(image)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]int{{4, 4}, {64, 48}, {123, 91}} {
		r, g, b, a := img.At(p[0], p[1]).RGBA()
		if r>>8 < 0xc0 || g>>8 > 0x40 || b>>8 > 0x50 || a>>8 < 0xf0 {
			t.Errorf("pixel %v = %02x%02x%02x/%02x, want the app's d02030", p, r>>8, g>>8, b>>8, a>>8)
		}
	}
	var left int
	onMain(func() {
		for s := range surfaces {
			if !s.closed {
				left++
			}
		}
	})
	if left != 0 {
		t.Errorf("%d surfaces left open after the thumbnail", left)
	}
}

func TestQuickLookThumbnailTimesOut(t *testing.T) {
	var got QuickLookThumbnailRequest
	onMain(func() { registerQuickLookThumbnailProvider(thumbnailFactory(&got), Options{}) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if image, err := renderQuickLookThumbnail(ctx, QuickLookThumbnailRequest{Path: "/tmp/x", Size: driver.Size{Width: 8, Height: 8}, Scale: 1}); err == nil || image != 0 {
		t.Fatalf("an ended context drew %v, err %v; want an error", image, err)
	}
}

func TestQuickLookThumbnailProviderReplies(t *testing.T) {
	sample := filepath.Join(t.TempDir(), "scene.scenra")
	if err := os.WriteFile(sample, []byte("scene"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got QuickLookThumbnailRequest
	onMain(func() {
		cls := registerQuickLookThumbnailProvider(thumbnailFactory(&got), Options{})
		super := apple.Class("QLThumbnailProvider")
		if super == 0 || !objc.Send[bool](objc.ID(cls), sel("isSubclassOfClass:"), objc.ID(super)) {
			t.Error("NSGoQuickLookThumbnailProvider is no QLThumbnailProvider")
		}
		if !objc.Send[bool](objc.ID(cls), sel("instancesRespondToSelector:"), sel("provideThumbnailForFileRequest:completionHandler:")) {
			t.Error("NSGoQuickLookThumbnailProvider does not provide thumbnails")
		}
	})
	for _, tc := range []struct {
		name  string
		path  string
		reply bool
	}{{"file", sample, true}, {"no URL", "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan struct{})
			var reply, failure objc.ID
			handler := objcblock.New(func(_ objc.Block, r, err objc.ID) {
				reply, failure = r, err
				close(done)
			})
			defer handler.Release()
			// QuickLook asks from a thread of its own.
			go apple.WithPool(func() {
				var url objc.ID
				if tc.path != "" {
					url = objc.ID(apple.Class("NSURL")).Send(sel("fileURLWithPath:"), apple.NSString(tc.path))
				}
				provideQuickLookThumbnail(url, apple.CGSize{Width: 32, Height: 32}, 2, objc.ID(handler))
			})
			pumpMainUntil(t, done)
			if tc.reply && (reply == 0 || failure != 0) {
				t.Fatalf("reply %v, error %v; want a reply", reply, failure)
			}
			if !tc.reply && (reply != 0 || failure == 0) {
				t.Fatalf("reply %v, error %v; want an error", reply, failure)
			}
			if tc.reply && got.Path != sample {
				t.Errorf("path = %q, want %q", got.Path, sample)
			}
		})
	}
}
