package nsgotest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Frames follow the manual clock: Advance runs the frame an app waits for
// at each display interval, with the frame's time from that clock.
func TestFramesFollowTheClock(t *testing.T) {
	var times []time.Time
	app := Start(t, func(cx *core.Scope) core.Node {
		var loop func(at time.Time)
		loop = func(at time.Time) {
			times = append(times, at)
			cx.RequestAnimationFrame(loop)
		}
		cx.RequestAnimationFrame(loop)
		return core.View{}
	})
	app.Advance(3 * driver.FrameInterval)
	if len(times) != 3 {
		t.Fatalf("the callback ran %d times in three frames, want 3", len(times))
	}
	for i := 1; i < len(times); i++ {
		if got := times[i].Sub(times[i-1]); got != driver.FrameInterval {
			t.Fatalf("frame %d came %v after the one before, want %v", i, got, driver.FrameInterval)
		}
	}
	if !times[2].Equal(app.Clock.Now()) {
		t.Fatalf("the last frame's time = %v, want the clock's %v", times[2], app.Clock.Now())
	}
}

func TestEnvironmentReportsMinimized(t *testing.T) {
	var states []bool
	app := Start(t, func(cx *core.Scope) core.Node {
		cx.Effect(func() { states = append(states, desktop.Minimized(cx)) })
		return core.View{}
	})
	app.SetEnvironment(core.Environment{Minimized: true})
	if len(states) != 2 || states[0] || !states[1] {
		t.Fatalf("minimized states %v", states)
	}
}

// TestHotKeyRegistrationAnswers: the headless platform registers an app's global shortcuts, answers a combination the app holds already as taken, and those HotKeyTaken and HotKeyUnsupported list as they say, so a test sees what the app shows for each.
func TestHotKeyRegistrationAnswers(t *testing.T) {
	if Target() != "" {
		t.Skip("a platform answers the shortcuts itself")
	}
	recorder := func(cx *core.Scope) core.Node {
		first := cx.RegisterHotKey("Ctrl+Alt+K", nil)
		again := cx.RegisterHotKey("ctrl+alt+k", nil)
		other := cx.RegisterHotKey("Alt+Space", nil)
		return core.View{Children: []core.Node{
			core.Text{ContentFunc: func() string { return "first " + first.State().String() }},
			core.Text{ContentFunc: func() string { return "again " + again.State().String() }},
			core.Text{ContentFunc: func() string { return "other " + other.State().String() }},
		}}
	}
	app := Start(t, recorder)
	app.Find("text:first registered")
	app.Find("text:again taken")
	app.Find("text:other registered")

	app = Start(t, recorder, HotKeyTaken("ctrl+alt+k"), HotKeyUnsupported("alt + space"))
	app.Find("text:first taken")
	app.Find("text:again taken")
	app.Find("text:other unsupported")
}

// Headless there are no native pixels: Capture and CapturePNG skip the test, and Screenshot only logs, but still fails on a name that leaves the screenshots directory.
func TestCaptureSkipsHeadlessAndScreenshotNamesStayInside(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless behavior")
	}
	root := func(cx *core.Scope) core.Node { return core.View{} }
	for name, capture := range map[string]func(*App){
		"Capture":    func(a *App) { a.Capture() },
		"CapturePNG": func(a *App) { a.CapturePNG() },
	} {
		var sub *testing.T
		t.Run(name, func(t *testing.T) {
			sub = t
			capture(Start(t, root))
			t.Error("captured headless")
		})
		if !sub.Skipped() {
			t.Errorf("%s did not skip the test headless", name)
		}
	}
	for _, name := range []string{"", ".", "./", "../outside", "en/../../outside", filepath.Join(os.TempDir(), "outside")} {
		if msg := fails(t, func(tb testing.TB) { Start(tb, root).Screenshot(name) }); !strings.Contains(msg, "relative file path") {
			t.Errorf("Screenshot(%q) failed with %q, want a relative file path asked for", name, msg)
		}
	}
	if msg := fails(t, func(tb testing.TB) { Start(tb, root).Screenshot("en/my library") }); msg != "" {
		t.Errorf("Screenshot of a relative name failed headless: %s", msg)
	}
}

// The model records reveals and trashes, answers them as the test says, and passes core.ErrNoTrash on as itself.
func TestRevealAndTrashAnswerAsTheTestSays(t *testing.T) {
	var cx *core.Scope
	app := Start(t, func(c *core.Scope) core.Node { cx = c; return core.View{} })
	path := filepath.Join(t.TempDir(), "film.mp4")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var revealed error = errors.New("no answer")
	cx.RevealFile(path, func(err error) { revealed = err })
	app.Settle()
	if revealed != nil || len(app.RevealedFiles) != 1 || app.RevealedFiles[0] != path {
		t.Fatalf("reveal answered %v, recorded %q", revealed, app.RevealedFiles)
	}
	var at string
	var trashed error
	trash := func() {
		at, trashed = "unanswered", nil
		cx.TrashFile(path, func(a string, err error) { at, trashed = a, err })
		app.Settle()
	}
	trash()
	if at != "" || trashed != nil {
		t.Fatalf("default trash answered %q, %v", at, trashed)
	}
	app.TrashFile = func(string) (string, error) { return "/Trash/film.mp4", nil }
	trash()
	if at != "/Trash/film.mp4" || trashed != nil {
		t.Fatalf("trash answered %q, %v", at, trashed)
	}
	app.TrashFile = func(string) (string, error) { return "", core.ErrNoTrash }
	trash()
	if !errors.Is(trashed, core.ErrNoTrash) {
		t.Fatalf("no trash answered %v", trashed)
	}
	if len(app.TrashedFiles) != 3 {
		t.Fatalf("recorded %q", app.TrashedFiles)
	}
}
