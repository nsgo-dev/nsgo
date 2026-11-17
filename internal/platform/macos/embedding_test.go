//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"
)

func TestEmbeddedViewCloseIsIdempotent(t *testing.T) {
	for _, stopFirst := range []bool{false, true} {
		name := "live surface"
		if stopFirst {
			name = "stopped surface"
		}
		t.Run(name, func(t *testing.T) {
			onMain(func() {
				loadAppKit()
				parent := objc.ID(classView).Send(selNew)
				defer parent.Send(selRelease)
				// No window is needed to exercise native ownership: the
				// host may close a view before its engine starts, too.
				v := NewView(nil, Options{Width: 160, Height: 100})
				defer v.Close()
				native := objc.ID(v.Handle())
				parent.Send(selAddSubview, native)
				if stopFirst {
					v.s.close()
				}
				v.Close()
				if !v.s.closed || surfaces[v.s] || views[native] != nil {
					t.Error("Close left the surface or native view registered")
				}
				if v.Handle() != 0 {
					t.Error("Close left a native handle to the released view")
				}
				if count := parent.Send(sel("subviews")).Send(sel("count")); count != 0 {
					t.Errorf("Close left %d views in the host", count)
				}
				// The second explicit call and deferred call must neither
				// message nor release the native view again.
				v.Close()
			})
		})
	}
}
