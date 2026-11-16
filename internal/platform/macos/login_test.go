//go:build darwin && !ios

package macos

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func TestLaunchAgentRegistration(t *testing.T) {
	a := launchAgent{filepath.Join(t.TempDir(), "LaunchAgents", "com.example.test.plist"), "com.example.test", "/Applications/A & B's <App>.app/Contents/MacOS/App"}
	if a.enabled() {
		t.Fatal("a missing login item is enabled")
	}
	for range 2 {
		if err := a.set(true); err != nil || !a.enabled() {
			t.Fatalf("enable login item: %v", err)
		}
	}
	// Let the system's plist reader check escaping and argument boundaries.
	data, err := exec.Command("/usr/bin/plutil", "-extract", "ProgramArguments", "json", "-o", "-", a.path).CombinedOutput()
	if err != nil {
		t.Fatalf("plutil: %v: %s", err, data)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil || !slices.Equal(args, []string{a.executable, "--nsgo-launched-at-login"}) {
		t.Fatalf("program arguments: %q, %v", args, err)
	}
	for range 2 {
		if err := a.set(false); err != nil || a.enabled() {
			t.Fatalf("disable login item: %v", err)
		}
	}
	if err := os.WriteFile(a.path, []byte("invalid plist"), 0o600); err != nil {
		t.Fatal(err)
	}
	if a.enabled() {
		t.Fatal("an invalid login item is enabled")
	}
	// A failed replacement preserves the file that was already there.
	a.path = filepath.Join(a.path, "cannot-write.plist")
	if err := a.set(true); err == nil {
		t.Fatal("enabling a login item in an unwritable location succeeded")
	}
}

func TestLoginLaunchAppleEvent(t *testing.T) {
	onMain(func() {
		loadAppKit()
		cls := objc.ID(apple.Class("NSAppleEventDescriptor"))
		for _, c := range []struct {
			name         string
			kind, reason uint32
			want         bool
		}{
			{"login", 0x6f617070, 0x6c676974, true}, // oapp, lgit
			{"normal", 0x6f617070, 0, false},
			{"service", 0x6f617070, 0x73766974, false}, // svit
			{"reopen", 0x72617070, 0x6c676974, false},  // rapp
		} {
			event := cls.Send(sel("appleEventWithEventClass:eventID:targetDescriptor:returnID:transactionID:"), uint32(0x61657674), c.kind, objc.ID(0), int16(-1), int32(0))
			if c.reason != 0 {
				event.Send(sel("setParamDescriptor:forKeyword:"), cls.Send(sel("descriptorWithEnumCode:"), c.reason), uint32(0x70726474)) // prdt
			}
			if got := isLoginLaunchEvent(event); got != c.want {
				t.Errorf("%s: login launch = %v, want %v", c.name, got, c.want)
			}
		}
		if isLoginLaunchEvent(0) {
			t.Error("missing event reported a login launch")
		}
	})
}

// Each child owns a fresh NSApplication, as a packaged launch does. The app sees the login reason when its function first runs, and the files and links AppKit hands it before applicationDidFinishLaunching:, before the engine starts, reach the handlers it registers.
func TestLoginLaunchBeforeAppStarts(t *testing.T) {
	mode := os.Getenv("NSGO_TEST_LOGIN_RUN")
	if mode == "" {
		for _, mode := range []string{"normal", "login"} {
			t.Run(mode, func(t *testing.T) {
				cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLoginLaunchBeforeAppStarts$")
				cmd.Env = append(os.Environ(), "NSGO_TEST_LOGIN_RUN="+mode)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("launch: %v\n%s", err, out)
				}
			})
		}
		return
	}
	path := filepath.Join(t.TempDir(), "opened.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	const link = "nsgo-login-test://open/item"
	var gotLogin bool
	var gotPaths, gotLinks []string
	watchdog := time.AfterFunc(20*time.Second, func() { panic("login launch did not reach the app") })
	defer watchdog.Stop()
	onMain(func() {
		loadAppKit()
		center := objc.ID(apple.Class("NSNotificationCenter")).Send(sel("defaultCenter"))
		stop := apple.Observe(center, apple.Global(appKitPath, "NSApplicationWillFinishLaunchingNotification"), func(objc.ID) {
			file := objc.ID(apple.Class("NSURL")).Send(sel("fileURLWithPath:"), apple.NSString(path))
			scheme := objc.ID(apple.Class("NSURL")).Send(sel("URLWithString:"), apple.NSString(link))
			items := [2]objc.ID{file, scheme}
			urls := objc.ID(apple.Class("NSArray")).Send(sel("arrayWithObjects:count:"), unsafe.Pointer(&items[0]), uint(len(items)))
			nsApp().Send(sel("delegate")).Send(sel("application:openURLs:"), nsApp(), urls)
		})
		defer stop()
		Run(core.AppRoot(func(cx *core.Scope) {
			gotLogin = cx.Env().LaunchedAtLogin()
			quitWhenBoth := func() {
				if gotPaths != nil && gotLinks != nil {
					cx.Quit()
				}
			}
			cx.OnOpenFiles(func(paths []string) { gotPaths = paths; quitWhenBoth() })
			cx.OnOpenLinks(func(links []string) { gotLinks = links; quitWhenBoth() })
		}), Options{Title: "Login launch", HideDock: true, LaunchedAtLogin: mode == "login"})
	})
	if gotLogin != (mode == "login") || !slices.Equal(gotPaths, []string{path}) || !slices.Equal(gotLinks, []string{link}) {
		t.Fatalf("login = %v, files = %v, links = %v; want login = %v, files = [%s], links = [%s]", gotLogin, gotPaths, gotLinks, mode == "login", path, link)
	}
}
