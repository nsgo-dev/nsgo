//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestEmbeddedViewCloseIsIdempotent closes an embedded view twice, while its app runs, after its surface stopped and after its engine ended on a fault, and checks that the native view is released exactly once.
func TestEmbeddedViewCloseIsIdempotent(t *testing.T) {
	for _, end := range []string{"running", "stopped", "destroyed"} {
		t.Run(end, func(t *testing.T) {
			onMain(func() {
				loadAppKit()
				// Each step drains its own pool, so the retain counts below hold only real references.
				drained := func(fn func()) {
					pool := objc.ID(apple.Class("NSAutoreleasePool")).Send(selNew)
					fn()
					pool.Send(sel("drain"))
				}
				var parent, native objc.ID
				var v *View
				drained(func() {
					parent = objc.ID(classView).Send(selNew)
					// No window is needed to exercise native ownership: the host may close a view before its engine starts, too.
					v = NewView(nil, Options{Width: 160, Height: 100})
					native = objc.ID(v.Handle())
					parent.Send(selAddSubview, native)
					// The test keeps a reference of its own, so a release too many shows in the retain count instead of freeing the view under the test.
					native.Send(sel("retain"))
				})
				defer parent.Send(selRelease)
				retained := func() uint { return uint(native.Send(sel("retainCount"))) }
				inHost := func() bool { return parent.Send(sel("subviews")).Send(sel("count")) == 1 }
				// The view's own reference, the host's and the test's.
				start := retained()
				drained(func() {
					switch end {
					case "stopped":
						v.s.close()
					case "destroyed":
						// The batch an engine that ends on a fault sends its root surface.
						v.s.apply(&driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemWindow, Window: &driver.WindowCommand{Action: driver.WindowDestroy}}}})
					}
				})
				var hosted bool
				drained(func() { hosted = inHost() })
				want := start
				if !hosted {
					want--
				}
				if got := retained(); got != want {
					// The view is gone or about to go: leak it rather than crash the process.
					t.Errorf("the app's end released the view the host still holds: retain count %d, want %d", got, want)
					return
				}
				defer native.Send(selRelease)
				drained(v.Close)
				// Close drops the view's own reference and the host's.
				want = start - 2
				if got := retained(); got != want {
					t.Errorf("Close left the view's retain count at %d, want %d", got, want)
				}
				if !v.s.closed || surfaces[v.s] || views[native] != nil {
					t.Error("Close left the surface or native view registered")
				}
				if v.Handle() != 0 {
					t.Error("Close left a native handle to the released view")
				}
				var count objc.ID
				drained(func() { count = parent.Send(sel("subviews")).Send(sel("count")) })
				if count != 0 {
					t.Errorf("Close left %d views in the host", count)
				}
				drained(v.Close)
				if got := retained(); got != want {
					t.Errorf("a second Close changed the view's retain count from %d to %d", want, got)
				}
			})
		})
	}
}
