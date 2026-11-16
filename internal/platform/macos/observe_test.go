//go:build darwin && !ios

package macos

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Each child creates and closes many windows in one process; fresh children
// also exercise the observers installed only when the first window opens.
func TestWindowObserversSurviveGC(t *testing.T) {
	if os.Getenv("NSGO_OBSERVER_STRESS_CHILD") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		for child := range 6 {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestWindowObserversSurviveGC$", "-test.timeout=25s")
			cmd.Env = append(os.Environ(), "NSGO_OBSERVER_STRESS_CHILD=1", "GOGC=1", "GOTRACEBACK=all")
			output, err := cmd.CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("observer stress child %d: %v\n%s", child, err, output)
			}
		}
		return
	}
	for iteration := range 48 {
		var callbacks int
		onMain(func() {
			loadAppKit()
			pool := objc.ID(apple.Class("NSAutoreleasePool")).Send(selNew)
			defer pool.Send(sel("drain"))
			window := objc.ID(classWindow).Send(selAlloc).Send(sel("initWithContentRect:styleMask:backing:defer:"),
				apple.Rect(0, 0, 160, 100), uint(0), uint(2), false)
			window.Send(sel("setReleasedWhenClosed:"), false)
			s := newSurface(Options{Width: 160, Height: 100}, false)
			s.window = window
			window.Send(sel("setContentView:"), s.native)
			s.observe()
			center := objc.ID(apple.Class("NSNotificationCenter")).Send(sel("defaultCenter"))
			name := apple.NSString("NSGoObserverStress")
			stop := apple.Observe(center, name, func(objc.ID) { callbacks++ })
			runtime.GC()
			center.Send(sel("postNotificationName:object:"), name, window)
			stop()
			stop()
			center.Send(sel("postNotificationName:object:"), name, window)
			s.close()
			delete(views, s.native)
			s.native.Send(selRelease)
			window.Send(sel("close"))
			window.Send(selRelease)
		})
		if callbacks != 1 {
			t.Fatalf("window %d: observer called %d times, want one before removal", iteration, callbacks)
		}
	}
}
