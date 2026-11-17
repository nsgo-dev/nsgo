//go:build windows

package windows

import "testing"

func TestCoversTellsFullScreenFromSmaller(t *testing.T) {
	monitor := rect{Left: 1920, Top: 0, Right: 3840, Bottom: 1080}
	for _, tc := range []struct {
		name string
		r    rect
		want bool
	}{
		{"the monitor", monitor, true},
		{"past its edges", rect{Left: 1912, Top: -8, Right: 3848, Bottom: 1088}, true},
		{"above the taskbar", rect{Left: 1920, Top: 0, Right: 3840, Bottom: 1032}, false},
		{"on another monitor", rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}, false},
	} {
		if got := covers(tc.r, monitor); got != tc.want {
			t.Errorf("%s: covers = %v, want %v", tc.name, got, tc.want)
		}
	}
	if covers(rect{Right: 10, Bottom: 10}, rect{}) {
		t.Error("covers an empty monitor")
	}
}

func TestHex4(t *testing.T) {
	if got := hex4(0x0409) + hex4(0x04b0); got != "040904b0" {
		t.Errorf("hex4 = %q", got)
	}
}
