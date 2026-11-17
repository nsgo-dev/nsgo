//go:build linux && !android

package linux

import "testing"

func TestWMClassSplitsInstanceAndClass(t *testing.T) {
	for _, tc := range []struct {
		raw             string
		instance, class string
	}{
		{"gnome-terminal-server\x00Gnome-terminal\x00", "gnome-terminal-server", "Gnome-terminal"},
		{"code\x00Code\x00", "code", "Code"},
		{"only\x00", "only", ""},
		{"", "", ""},
	} {
		if instance, class := wmClass([]byte(tc.raw)); instance != tc.instance || class != tc.class {
			t.Errorf("wmClass(%q) = %q, %q, want %q, %q", tc.raw, instance, class, tc.instance, tc.class)
		}
	}
}
