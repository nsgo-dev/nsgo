//go:build linux && !android

package linux

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
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

// TestEndAwakeWaitsForTheReleases sends every hold's release before a quitting app's main loop returns, as the process exits right after and would drop releases only queued, and bounds the wait by a queue that does not answer.
func TestEndAwakeWaitsForTheReleases(t *testing.T) {
	w := &window{}
	var ended atomic.Bool
	ready := make(chan struct{})
	portalQueue() <- func() {
		awake.holds = map[awakeKey]bool{{w, 1}: false}
		awake.groups[0] = awakeGroup{refs: 1, end: func() {
			time.Sleep(50 * time.Millisecond) // a synchronous Uninhibit on the bus
			ended.Store(true)
		}}
		close(ready)
	}
	<-ready
	awakeTaken.Store(true)
	t.Cleanup(func() { awakeTaken.Store(false) })
	if !endAwake(5*time.Second) || !ended.Load() {
		t.Fatal("endAwake returned before the computer-only hold's release was sent")
	}
	if len(awake.holds) != 0 || awake.groups[0].refs != 0 || awake.groups[0].end != nil {
		t.Fatalf("endAwake left holds %v and group %+v", awake.holds, awake.groups[0])
	}

	stuck := make(chan struct{})
	portalQueue() <- func() { <-stuck }
	start := time.Now()
	if endAwake(100 * time.Millisecond) {
		t.Fatal("endAwake reported the releases sent behind a call that never returned")
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Fatalf("endAwake waited %v behind a stuck queue, past its bound", waited)
	}
	close(stuck)
}
