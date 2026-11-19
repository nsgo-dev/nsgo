package backend

import (
	"bytes"
	"log"
	"slices"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestLaunchFlags(t *testing.T) {
	var f launchFlags
	got := f.parse([]string{"app", "--nsgo-fps", "-v", "-nsgo-lang=zh-Hans", "--nsgo-appearance=dark", "--nsgo-nope", "file.txt", "--", "--nsgo-fps=false"})
	if want := []string{"app", "-v", "file.txt", "--", "--nsgo-fps=false"}; !slices.Equal(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	if !f.fps || f.lang != "zh-Hans" || !f.appearanceSet || f.appearance != core.AppearanceDark {
		t.Errorf("flags = %+v", f)
	}

	f = launchFlags{}
	f.parse([]string{"app", "--nsgo-fps=false", "--nsgo-lang", "--nsgo-appearance=blue"})
	if f.fps || f.lang != "" || f.appearanceSet {
		t.Errorf("malformed flags = %+v", f)
	}

	o := Options{Appearance: core.AppearanceLight}
	(&launchFlags{fps: true, appearanceSet: true}).apply(&o)
	if !o.Monitor || o.Appearance != core.AppearanceSystem {
		t.Errorf("options = %+v", o)
	}
	if got := f.parse(nil); got != nil {
		t.Errorf("no args = %q", got)
	}
}

// Windows' installer and COM start the app with flags of their own, which the app's flag parsing never sees, COM's -Embedding included.
func TestToastLaunchFlags(t *testing.T) {
	var f launchFlags
	got := f.parse([]string{`C:\App\App.exe`, `--nsgo-register-toast=C:\Start Menu\Bob's Notes.lnk`, "--nsgo-unregister-toast"})
	if want := []string{`C:\App\App.exe`}; !slices.Equal(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	if f.registerToast != `C:\Start Menu\Bob's Notes.lnk` || !f.unregisterToast || f.toastActivation {
		t.Errorf("flags = %+v", f)
	}

	f = launchFlags{}
	got = f.parse([]string{"app", "--nsgo-toast-activation", "-Embedding", "-v"})
	if want := []string{"app", "-v"}; !slices.Equal(got, want) || !f.toastActivation {
		t.Errorf("args = %q, want %q, flags = %+v", got, want, f)
	}
	f = launchFlags{}
	if got, want := f.parse([]string{"app", "--nsgo-toast-activation", "/Embedding"}), []string{"app"}; !slices.Equal(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	// Without the activation flag -Embedding is the app's.
	f = launchFlags{}
	if got, want := f.parse([]string{"app", "-Embedding", "--nsgo-register-toast"}), []string{"app", "-Embedding"}; !slices.Equal(got, want) || f.registerToast != "" {
		t.Errorf("args = %q, want %q, flags = %+v", got, want, f)
	}
}

// A Windows Jump List task starts the app with the label of the item it chooses, which the app's flag parsing never sees and a restart does not choose again.
func TestDockTaskLaunchFlag(t *testing.T) {
	var f launchFlags
	args := []string{`C:\App\App.exe`, "--nsgo-dock-menu=New Window", "notes.txt"}
	if got, want := f.parse(args), []string{`C:\App\App.exe`, "notes.txt"}; !slices.Equal(got, want) || f.dockTask != "New Window" {
		t.Errorf("args = %q, want %q, flags = %+v", got, want, f)
	}
	if got, want := restartArgs(append(args, "-nsgo-dock-menu=Other", "--nsgo-fps")), []string{"notes.txt", "--nsgo-fps"}; !slices.Equal(got, want) {
		t.Errorf("restart args = %q, want %q", got, want)
	}
	if got := restartArgs([]string{"app"}); got != nil {
		t.Errorf("restart args without any = %q", got)
	}
	f = launchFlags{}
	f.parse([]string{"app", "--nsgo-dock-menu="})
	if f.dockTask != "" {
		t.Errorf("an empty task = %+v", f)
	}
}

func TestLoginLaunchFlag(t *testing.T) {
	var f launchFlags
	got := f.parse([]string{"app", "--nsgo-launched-at-login", "--user-option"})
	if !f.launchedAtLogin || !slices.Equal(got, []string{"app", "--user-option"}) {
		t.Fatalf("login launch flag = %v, args = %q", f.launchedAtLogin, got)
	}
	f = launchFlags{}
	got = f.parse([]string{"app", "--", "--nsgo-launched-at-login"})
	if f.launchedAtLogin || !slices.Equal(got, []string{"app", "--", "--nsgo-launched-at-login"}) {
		t.Fatalf("app-owned flag changed launch reason: %v, args = %q", f.launchedAtLogin, got)
	}
}

// A restart is the user's, not the login's: it drops the marker the LaunchAgent started the app with, and keeps the app's own arguments.
func TestRestartDropsTheLoginMarker(t *testing.T) {
	got := restartArgs([]string{"app", "--nsgo-launched-at-login", "-v", "-nsgo-launched-at-login", "--nsgo-fps", "--", "--nsgo-launched-at-login"})
	if want := []string{"-v", "--nsgo-fps", "--", "--nsgo-launched-at-login"}; !slices.Equal(got, want) {
		t.Errorf("restart args = %q, want %q", got, want)
	}
	if got := restartArgs(nil); got != nil {
		t.Errorf("no args = %q", got)
	}
}

// A link asked for something once, as a Jump List task does: a restart drops it, also after "--", where Windows' registration of a URL scheme passes it.
func TestRestartDropsLinks(t *testing.T) {
	got := restartArgs([]string{"app", "umapp://connect?key=1", "notes.txt", "--", "umapp://settings", "--app-flag"})
	if want := []string{"notes.txt", "--", "--app-flag"}; !slices.Equal(got, want) {
		t.Errorf("restart args = %q, want %q", got, want)
	}
}

// --nsgo-os names the OS a web build runs as; other builds say they ignore it.
func TestOSLaunchFlag(t *testing.T) {
	for value, want := range map[string]core.OS{"macos": core.OSMacOS, "windows": core.OSWindows, "linux": core.OSLinux, "ios": core.OSIOS, "android": core.OSAndroid, "web": core.OSWeb} {
		var f launchFlags
		if got := f.parse([]string{"app", "--nsgo-os=" + value, "-v"}); !slices.Equal(got, []string{"app", "-v"}) || f.os != want {
			t.Errorf("--nsgo-os=%s: args = %q, os = %q", value, got, f.os)
		}
	}
	var f launchFlags
	if err := f.set("os=beos"); err == nil || f.os != "" {
		t.Errorf("--nsgo-os=beos: err = %v, os = %q", err, f.os)
	}
	if err := f.set("os"); err == nil || f.os != "" {
		t.Errorf("--nsgo-os: err = %v, os = %q", err, f.os)
	}
	if err := f.set("nope"); err == nil || !strings.Contains(err.Error(), "--nsgo-appearance or --nsgo-os") {
		t.Errorf("an unknown flag's error = %v", err)
	}

	var out bytes.Buffer
	defer log.SetOutput(log.Writer())
	log.SetOutput(&out)
	defer func(was bool) { simulates = was }(simulates)
	for _, web := range []bool{false, true} {
		out.Reset()
		simulates = web
		var o Options
		(&launchFlags{os: core.OSIOS}).apply(&o)
		if o.Simulate != core.OSIOS {
			t.Errorf("web %v: Simulate = %q", web, o.Simulate)
		}
		if warned := strings.Contains(out.String(), "only a web build"); warned == web {
			t.Errorf("web %v: logged %q", web, out.String())
		}
	}
	simulates = false
	for _, os := range []core.OS{"", core.OSWeb} {
		out.Reset()
		(&launchFlags{os: os}).apply(&Options{})
		if out.Len() != 0 {
			t.Errorf("--nsgo-os=%q, logged %q", os, out.String())
		}
	}
}
