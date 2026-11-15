//go:build linux && !android

package linux

import (
	"os"
	"testing"
)

// TestAwakeAppIDFallsBack names an app built without nsgo.yaml's id, as go build and go run make it, by its name, else its executable: GNOME's inhibitors refuse an empty ID.
func TestAwakeAppIDFallsBack(t *testing.T) {
	for _, c := range []struct{ id, name, exe, want string }{
		{"dev.nsgo.gallery", "Gallery", "/usr/bin/gallery", "dev.nsgo.gallery"},
		{"", "Gallery", "/usr/bin/gallery", "Gallery"},
		{"", "", "/tmp/go-build123/b001/exe/hello", "hello"},
		{"", "", "", "nsgo"},
		{"", "", "/", "nsgo"},
	} {
		if got := awakeAppID(c.id, c.name, c.exe); got != c.want {
			t.Errorf("awakeAppID(%q, %q, %q) = %q, want %q", c.id, c.name, c.exe, got, c.want)
		}
	}
}

// TestAwakeReasonIsNeverEmpty gives an inhibitor a reason when the app gives none: gnome-session refuses an empty one.
func TestAwakeReasonIsNeverEmpty(t *testing.T) {
	if got := awakeReason("Exporting", "Gallery"); got != "Exporting" {
		t.Errorf("reason %q", got)
	}
	if got := awakeReason("", "Gallery"); got != "Gallery" {
		t.Errorf("empty reason became %q", got)
	}
}

// TestAwakeServiceChoice sends a computer-only hold of an unsandboxed app straight to GNOME's session manager, whose portal backend drops an unsandboxed app's inhibition while answering that it holds, and every other computer-only hold to the portal; a display hold inhibits the screen saver.
func TestAwakeServiceChoice(t *testing.T) {
	for _, c := range []struct {
		display, sandboxed, gnome bool
		want                      awakeVia
	}{
		{display: false, sandboxed: false, gnome: true, want: viaSessionManager},
		{display: false, sandboxed: false, gnome: false, want: viaPortal},
		{display: false, sandboxed: true, gnome: true, want: viaPortal},
		{display: false, sandboxed: true, gnome: false, want: viaPortal},
		{display: true, sandboxed: false, gnome: true, want: viaScreenSaver},
		{display: true, sandboxed: true, gnome: false, want: viaScreenSaver},
	} {
		if got := awakeService(c.display, c.sandboxed, c.gnome); got != c.want {
			t.Errorf("awakeService(display %v, sandboxed %v, gnome %v) = %v, want %v", c.display, c.sandboxed, c.gnome, got, c.want)
		}
	}
}

// TestAwakeSandboxDetection counts a Snap as sandboxed by the environment snapd gives its apps.
func TestAwakeSandboxDetection(t *testing.T) {
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		t.Skip("running in a Flatpak")
	}
	t.Setenv("SNAP", "")
	if sandboxed() {
		t.Error("an app outside Flatpak and Snap counted as sandboxed")
	}
	t.Setenv("SNAP", "/snap/gallery/12")
	if !sandboxed() {
		t.Error("a Snap counted as unsandboxed")
	}
}
