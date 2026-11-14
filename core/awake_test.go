package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestKeepAwakeReleaseIsIdempotent(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	release := cx.KeepAwake("exporting", KeepAwakeOptions{Display: true})
	it := command(h.commit(), driver.ItemKeepAwake)
	if it == nil || it.Awake == nil || it.Count == 0 || it.Awake.Reason != "exporting" || !it.Awake.Display {
		t.Fatalf("hold = %+v", it)
	}
	release()
	release()
	h.drain()
	if it = command(h.commit(), driver.ItemReleaseAwake); it == nil || it.Count == 0 {
		t.Fatalf("release = %+v", it)
	}
	if command(h.commit(), driver.ItemReleaseAwake) != nil {
		t.Fatal("release was sent twice")
	}
}
