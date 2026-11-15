//go:build windows

package windows

import (
	"testing"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestReasonContextLayout keeps reasonContext at REASON_CONTEXT's size and offsets: the union starts after Version and Flags, and Detailed, its largest member, is a module handle, two ULONGs and a pointer.
func TestReasonContextLayout(t *testing.T) {
	ptr := unsafe.Sizeof(uintptr(0))
	var ctx reasonContext
	if got, want := unsafe.Sizeof(ctx), 8+2*ptr+8; got != want {
		t.Errorf("sizeof(REASON_CONTEXT) = %d, want %d", got, want)
	}
	if got := unsafe.Offsetof(ctx.reason); got != 8 {
		t.Errorf("SimpleReasonString's offset = %d, want 8", got)
	}
}

// TestAwakeHoldsSharePowerRequests gives every window's holds of one kind one power request, carrying a reason powercfg lists, and closes it with the kind's last hold, a destroyed window's included.
func TestAwakeHoldsSharePowerRequests(t *testing.T) {
	a, b := &window{}, &window{}
	t.Cleanup(func() {
		a.releaseAllAwake()
		b.releaseAllAwake()
	})
	computer, display := &awake.groups[0], &awake.groups[1]
	a.keepAwake(1, driver.KeepAwake{Reason: "Exporting"})
	if computer.request == 0 || computer.refs != 1 {
		t.Fatalf("a computer-only hold made request %#x with %d holds", computer.request, computer.refs)
	}
	request := computer.request
	b.keepAwake(1, driver.KeepAwake{Reason: "Syncing"})
	a.keepAwake(1, driver.KeepAwake{Reason: "Exporting"})
	if computer.request != request || computer.refs != 2 {
		t.Fatalf("a second window's hold made request %#x with %d holds, want %#x with 2", computer.request, computer.refs, request)
	}
	a.keepAwake(2, driver.KeepAwake{Reason: "Playing", Display: true})
	if display.request == 0 || display.request == request || display.refs != 1 {
		t.Fatalf("a display hold made request %#x with %d holds", display.request, display.refs)
	}
	a.releaseAllAwake()
	if display.request != 0 || display.refs != 0 {
		t.Fatalf("destroying the window left display request %#x with %d holds", display.request, display.refs)
	}
	if computer.request != request || computer.refs != 1 {
		t.Fatalf("destroying one window left computer request %#x with %d holds, want %#x with 1", computer.request, computer.refs, request)
	}
	b.releaseAwake(1)
	b.releaseAwake(1)
	if computer.request != 0 || computer.refs != 0 {
		t.Fatalf("releasing the last hold left computer request %#x with %d holds", computer.request, computer.refs)
	}
}
