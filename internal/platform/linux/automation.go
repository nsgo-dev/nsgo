//go:build linux && !android && nsgoautomation

package linux

import (
	"sync"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// liveWindows holds the standalone app's windows for the agent. Keep the
// same testSurface between requests so its input state follows the window.
var liveWindows sync.Map // uint32 -> *testSurface

func init() {
	testhost.RegisterLive(func(id uint32) testhost.Surface {
		if surface, ok := liveWindows.Load(id); ok {
			return surface.(*testSurface)
		}
		return nil
	})
}

func registerLiveWindow(w *window) {
	if w.opts.test == nil && w.kind != driver.SurfaceTrayIcon {
		liveWindows.Store(w.id, &testSurface{w: w})
	}
}

func unregisterLiveWindow(w *window) {
	if surface, ok := liveWindows.Load(w.id); ok && surface.(*testSurface).w == w {
		liveWindows.CompareAndDelete(w.id, surface)
	}
}
