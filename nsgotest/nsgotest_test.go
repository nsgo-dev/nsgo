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

// The app's directories are the test's own, made before the app starts, so a test never writes to the user's data, cache or logs.
func TestAppDirectoriesAreTemporary(t *testing.T) {
	if Target() == "web" {
		t.Skip("the web has no app directories")
	}
	var dirs []string
	Start(t, func(cx *core.Scope) core.Node {
		dirs = []string{cx.DataDir(), cx.CacheDir(), cx.LogsDir()}
		return core.View{}
	})
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		real, err := filepath.EvalSymlinks(d)
		if err != nil {
			t.Fatalf("%q: %v", d, err)
		}
		if fi, err := os.Stat(real); err != nil || !fi.IsDir() || !strings.HasPrefix(real, tmp+string(filepath.Separator)) || seen[real] {
			t.Fatalf("app directories %q: %q is not a directory of its own in %s", dirs, d, tmp)
		}
		seen[real] = true
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

// A drop on a view inside a drop zone reaches the zone, as a platform's hit test walks up to it; one with no zone around it, a disabled zone, or a zone outside the modal the view is in fails the step.
func TestDropFilesUsesTheNearestEnabledDropAncestor(t *testing.T) {
	var got []string
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{TestID: "zone", Style: core.Style{Width: core.Pt(100), Height: core.Pt(40)},
			OnFileDrop: func(uris []string) { got = append([]string(nil), uris...) },
			Children:   []core.Node{core.Text{Content: "drop here", TestID: "label"}}}
	})
	app.DropFiles("id:label", "file:///video.mp4")
	if len(got) != 1 || got[0] != "file:///video.mp4" {
		t.Fatalf("drop reached %q, want the ancestor", got)
	}
	if msg := fails(t, func(tb testing.TB) {
		Start(tb, func(cx *core.Scope) core.Node { return core.Text{Content: "elsewhere", TestID: "outside"} }).DropFiles("id:outside", "file:///a.mp4")
	}); !strings.Contains(msg, "no view hearing file drops matches") {
		t.Errorf("a drop with no zone failed with %q", msg)
	}
	if msg := fails(t, func(tb testing.TB) {
		app := Start(tb, func(cx *core.Scope) core.Node {
			return core.View{Disabled: func() bool { return true }, OnFileDrop: func([]string) { t.Error("a disabled zone took the drop") },
				Children: []core.Node{core.Text{Content: "drop here", TestID: "label"}}}
		})
		app.DropFiles("id:label", "file:///a.mp4")
	}); !strings.Contains(msg, "is disabled") {
		t.Errorf("a drop on a disabled zone failed with %q", msg)
	}
	if msg := fails(t, func(tb testing.TB) {
		app := Start(tb, func(cx *core.Scope) core.Node {
			return core.View{OnFileDrop: func([]string) { t.Error("a zone outside the modal took the drop") }, Children: []core.Node{
				core.Modal{Children: []core.Node{core.Text{Content: "in the modal", TestID: "inmodal"}}}}}
		})
		app.DropFiles("id:inmodal", "file:///a.mp4")
	}); !strings.Contains(msg, "no view hearing file drops matches") {
		t.Errorf("a drop in a modal failed with %q", msg)
	}
}

// Bounds is the frame Press aims at, in surface points: its ancestors' origins less the scroll offsets of the scroll views around it.
func TestViewBoundsUsesSurfaceCoordinates(t *testing.T) {
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(10)}, Children: []core.Node{
			core.View{TestID: "child", Style: core.Style{Width: core.Pt(40), Height: core.Pt(20)}},
			core.Scroll{TestID: "scroll", Style: core.Style{Height: core.Pt(100)}, Children: []core.Node{
				core.View{Style: core.Style{Height: core.Pt(300)}},
				core.View{TestID: "row", Style: core.Style{Height: core.Pt(30)}}}}}}
	})
	b := app.Find("id:child").Bounds()
	if b.Origin != (core.Point{X: 10, Y: 10}) || b.Size != (core.Size{Width: 40, Height: 20}) {
		t.Fatalf("bounds = %+v, want 10,10 40x20", b)
	}
	app.ScrollTo("id:scroll", core.Point{Y: 250})
	if b := app.Find("id:row").Bounds(); b.Origin.Y != 10+20+300-250 {
		t.Fatalf("scrolled row bounds = %+v, want y %v", b, 10+20+300-250)
	}
}

// Headless, the system switch takes a press itself, as on a target: it reports the value it flips to. A disabled one fails the press, and a hidden one matches nothing.
func TestPressTogglesSystemSwitchHeadless(t *testing.T) {
	for _, os := range []core.OS{core.OSMacOS, core.OSIOS} {
		var changes []bool
		checked := false
		disabled := false
		app := Start(t, func(cx *core.Scope) core.Node {
			on := core.NewSignal(cx, false)
			return core.SystemSwitch{Checked: func() bool { checked = on.Get(); return checked }, Disabled: func() bool { return disabled },
				OnChange: func(v bool) { changes = append(changes, v); on.Set(v) }, TestID: "switch"}
		}, Environment(core.Environment{OS: os}))
		app.Press("id:switch")
		app.Press("id:switch")
		if len(changes) != 2 || !changes[0] || changes[1] || checked {
			t.Fatalf("%v: presses reported %v, checked %v", os, changes, checked)
		}
	}
	if msg := fails(t, func(tb testing.TB) {
		Start(tb, func(cx *core.Scope) core.Node {
			return core.SystemSwitch{Disabled: func() bool { return true }, OnChange: func(bool) { t.Error("a disabled switch changed") }, TestID: "switch"}
		}, Environment(core.Environment{OS: core.OSMacOS})).Press("id:switch")
	}); !strings.Contains(msg, "disabled") {
		t.Errorf("a press on a disabled switch failed with %q", msg)
	}
	if msg := fails(t, func(tb testing.TB) {
		Start(tb, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Display: core.DisplayNone}, Children: []core.Node{
				core.SystemSwitch{OnChange: func(bool) { t.Error("a hidden switch changed") }, TestID: "switch"}}}
		}, Environment(core.Environment{OS: core.OSMacOS})).Press("id:switch")
	}); msg == "" {
		t.Error("a press on a hidden switch passed")
	}
}

// A web view that is mounted but hidden goes on loading and posting messages, as a platform's does.
func TestNavigateReportsToHiddenMountedWebView(t *testing.T) {
	var loaded []string
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Display: core.DisplayNone}, Children: []core.Node{
			core.WebView{TestID: "web", Source: core.WebSrc("https://example.com"),
				OnLoadStart: func(n core.WebNavigation) { loaded = append(loaded, "start:"+n.URL) },
				OnLoad:      func(n core.WebNavigation) { loaded = append(loaded, "load:"+n.URL) },
				OnMessage:   func(d string) { loaded = append(loaded, "message:"+d) }}}}
	})
	app.Navigate("id:web", core.WebNavigation{URL: "https://example.com/next"})
	app.PostMessage("id:web", "hi")
	if want := "start:https://example.com/next load:https://example.com/next message:hi"; strings.Join(loaded, " ") != want {
		t.Fatalf("web view events = %v, want %s", loaded, want)
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

// With a desktop environment the main view shows in the main window, whose close CloseWindow asks for: its handlers run newest first, and it closes unless one cancels.
func TestCloseWindowRunsTheMainWindowsHandlers(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	var calls []string
	var w *desktop.Window
	cancel := true
	app := Start(t, func(cx *core.Scope) core.Node {
		w = desktop.CurrentWindow(cx)
		if w == nil {
			t.Fatal("the main view is in no window")
		}
		w.OnClosing(cx, func(c *desktop.WindowClosing) {
			calls = append(calls, "old")
			if cancel {
				c.Cancel()
			}
		})
		w.OnClosing(cx, func(c *desktop.WindowClosing) { calls = append(calls, "new") })
		return core.Text{Content: "main", TestID: "main"}
	}, Environment(core.Environment{OS: core.OSMacOS}))
	if app.root.MainWindow() != w {
		t.Fatal("the main view's window is not the main window")
	}
	if app.CloseWindow() {
		t.Fatal("a canceled close closed the window")
	}
	if got, want := strings.Join(calls, ","), "new,old"; got != want {
		t.Fatalf("closing handlers ran %q, want %q", got, want)
	}
	app.Find("id:main")
	cancel, calls = false, nil
	if !app.CloseWindow() || app.root.MainWindow() != nil {
		t.Fatal("an accepted close left the window open")
	}
	if len(app.Query("id:main")) != 0 {
		t.Fatal("steps still reach the closed window's views")
	}
	if msg := fails(t, func(tb testing.TB) { Start(tb, func(cx *core.Scope) core.Node { return core.View{} }).CloseWindow() }); !strings.Contains(msg, "no window to close") {
		t.Errorf("CloseWindow without a desktop environment failed with %q", msg)
	}
}

// Another window the app opens runs beside the main one without disturbing the views steps reach, and each window's alerts are answered to the window that showed them, though both number them alike.
func TestOtherWindowsLeaveTheMainOnesViews(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	var answers []string
	app := Start(t, func(cx *core.Scope) core.Node {
		desktop.OpenWindow(cx, desktop.WindowOptions{Width: 200, Height: 100, Title: "Other"}, func(cx *core.Scope) core.Node {
			cx.ShowAlert(core.AlertConfig{Title: "Other?", Actions: []core.AlertAction{{Label: "Yes"}}}, func(i int) { answers = append(answers, "other") })
			return core.View{TestID: "other", Children: []core.Node{core.Text{Content: "other window"}}}
		})
		cx.ShowAlert(core.AlertConfig{Title: "Main?", Actions: []core.AlertAction{{Label: "Yes"}}}, func(i int) { answers = append(answers, "main") })
		return core.View{TestID: "main", Children: []core.Node{core.Text{Content: "main window"}}}
	}, Environment(core.Environment{OS: core.OSMacOS}), PendingAlerts())
	app.Find("id:main")
	app.Find("text:main window")
	if len(app.Query("id:other")) != 0 {
		t.Fatal("steps reach another window's views")
	}
	if len(app.Alerts()) != 2 {
		t.Fatalf("alerts %v, want both windows'", app.Alerts())
	}
	first := app.Alerts()[0].Title
	app.Answer(0)
	app.Answer(0)
	want := "main other"
	if first == "Other?" {
		want = "other main"
	}
	if got := strings.Join(answers, " "); got != want {
		t.Fatalf("answers reached %q, want %q", got, want)
	}
}

// StartApp runs the app's function once in the app's own scope, as nsgo.Run does: what it registers lives as long as the app, and with a desktop environment the view it sets shows in the main window, whose steps reach it, while its application menu chooses in that scope.
func TestStartAppKeepsTheAppsScope(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	for _, os := range []core.OS{core.OSMacOS, core.OSWindows, core.OSLinux} {
		var disposed bool
		var chosen, quits int
		var w *desktop.Window
		app := StartApp(t, func(cx *core.Scope) {
			cx.Cleanup(func() { disposed = true })
			desktop.SetApplicationMenu(cx, []desktop.MenuItem{{Label: "File", Items: []desktop.MenuItem{{Label: "New", OnSelect: func() { chosen++ }}}}})
			desktop.OnQuitRequest(cx, func(*desktop.QuitRequest) { quits++ })
			cx.SetMainView(func(cx *core.Scope) core.Node {
				w = desktop.CurrentWindow(cx)
				return core.Text{Content: "main", TestID: "main"}
			})
		}, Environment(core.Environment{OS: os}))
		if disposed {
			t.Fatalf("%v: the app's scope was disposed as it started", os)
		}
		if w == nil || app.root.MainWindow() != w {
			t.Fatalf("%v: the main view is not in the main window", os)
		}
		app.Find("id:main")
		app.ChooseMenu("File", "New")
		if chosen != 1 {
			t.Fatalf("%v: the menu item ran %d times", os, chosen)
		}
		app.Report(driver.Event{Kind: driver.EventQuitRequest, Count: 1})
		if quits != 1 {
			t.Fatalf("%v: the quit handler ran %d times", os, quits)
		}
	}
	// Elsewhere the main view fills the app's one surface.
	app := StartApp(t, func(cx *core.Scope) {
		cx.SetMainView(func(cx *core.Scope) core.Node {
			if desktop.CurrentWindow(cx) != nil {
				t.Error("a phone opened a window")
			}
			return core.Text{Content: "main", TestID: "main"}
		})
	}, Environment(core.Environment{OS: core.OSIOS}))
	app.Find("id:main")
}

// Given a root view, Windows and Linux open no window, as nsgo.Run does there, and macOS shows it in the main window.
func TestStartOpensWindowsWhereRunDoes(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	for os, want := range map[core.OS]bool{core.OSMacOS: true, core.OSWindows: false, core.OSLinux: false} {
		var w *desktop.Window
		app := Start(t, func(cx *core.Scope) core.Node {
			w = desktop.CurrentWindow(cx)
			return core.Text{Content: "main", TestID: "main"}
		}, Environment(core.Environment{OS: os}))
		app.Find("id:main")
		if (w != nil) != want {
			t.Errorf("%v: the root view's window is %v, want one %v", os, w, want)
		}
	}
}

// Steps act on the window SetMainView opens, not on one the app opened before it, and on the next one once it closes and the app shows its view again.
func TestStepsFollowTheMainWindow(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	var show func()
	app := StartApp(t, func(cx *core.Scope) {
		desktop.OpenWindow(cx, desktop.WindowOptions{Width: 200, Height: 100, Title: "Settings"}, func(cx *core.Scope) core.Node {
			return core.Text{Content: "settings", TestID: "settings"}
		})
		show = func() {
			cx.SetMainView(func(cx *core.Scope) core.Node { return core.Text{Content: "main", TestID: "main"} })
		}
		show()
	}, Environment(core.Environment{OS: core.OSMacOS}))
	app.Find("id:main")
	if len(app.Query("id:settings")) != 0 {
		t.Fatal("steps reach the window the app opened first")
	}
	if !app.CloseWindow() {
		t.Fatal("the main window did not close")
	}
	app.Post(show)
	app.Settle()
	app.Find("id:main")
}

// Another window's shortcuts and menu bar leave the main window's, which Key and ChooseMenu reach, and a context menu any window shows chooses on that window.
func TestOtherWindowsKeepTheirShortcutsAndMenus(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	var got []string
	app := StartApp(t, func(cx *core.Scope) {
		cx.SetMainView(func(cx *core.Scope) core.Node {
			cx.OnKey("mod+s", func() { got = append(got, "save") })
			desktop.SetApplicationMenu(cx, []desktop.MenuItem{{Label: "File", Items: []desktop.MenuItem{{Label: "New", OnSelect: func() { got = append(got, "new") }}}}})
			return core.Text{Content: "main", TestID: "main"}
		})
		desktop.OpenWindow(cx, desktop.WindowOptions{Width: 200, Height: 100, Title: "Other"}, func(cx *core.Scope) core.Node {
			cx.OnKey("escape", func() { got = append(got, "escape") })
			desktop.SetApplicationMenu(cx, []desktop.MenuItem{{Label: "Other"}})
			desktop.PopupMenu(cx, []desktop.MenuItem{{Label: "Copy", OnSelect: func() { got = append(got, "copy") }}}, core.Point{}, func() { got = append(got, "closed") })
			return core.Text{Content: "other"}
		})
	}, Environment(core.Environment{OS: core.OSWindows}))
	if strings.Join(app.Keymap, " ") != "mod+s" {
		t.Fatalf("keymap %v, want the main window's", app.Keymap)
	}
	app.Key("mod+s")
	app.ChooseMenu("File", "New")
	app.ChoosePopup("Copy")
	if want := "save new copy closed"; strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
}

// The app icon's menu is the app's, the one a surface set last: ChooseDockMenu chooses on that surface, though the surfaces number their items alike, and the menu goes with the window that set it.
func TestDockMenuIsTheApps(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	var got []string
	var other *desktop.Window
	var open func()
	app := StartApp(t, func(cx *core.Scope) {
		desktop.SetDockMenu(cx, []desktop.MenuItem{{Label: "New Window", OnSelect: func() { got = append(got, "app") }}})
		cx.SetMainView(func(cx *core.Scope) core.Node {
			desktop.SetApplicationMenu(cx, []desktop.MenuItem{{Label: "File", Items: []desktop.MenuItem{{Label: "New", OnSelect: func() { got = append(got, "menu") }}}}})
			return core.Text{Content: "main", TestID: "main"}
		})
		open = func() {
			other = desktop.OpenWindow(cx, desktop.WindowOptions{Width: 200, Height: 100, Title: "Library"}, func(cx *core.Scope) core.Node {
				desktop.SetDockMenu(cx, []desktop.MenuItem{{Label: "Next Library", OnSelect: func() { got = append(got, "library") }}})
				return core.Text{Content: "library"}
			})
		}
	}, Environment(core.Environment{OS: core.OSMacOS}))
	app.Find("id:main")
	app.ChooseDockMenu("New Window")
	app.Post(open)
	app.Settle()
	if len(app.DockMenu) != 1 || app.DockMenu[0].Label != "Next Library" {
		t.Fatalf("dock menu %+v, want the library window's", app.DockMenu)
	}
	app.ChooseDockMenu("Next Library")
	app.Post(other.Close)
	app.Settle()
	if app.DockMenu != nil {
		t.Fatalf("dock menu %+v after its window closed", app.DockMenu)
	}
	if want := "app library"; strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
}

// Answering an alert by its place, as the automation does, answers it to the surface that showed it, though the surfaces number their alerts alike.
func TestAnswerAtReachesTheAlertsSurface(t *testing.T) {
	if Target() != "" {
		t.Skip("checks the headless model")
	}
	var answers []string
	app := StartApp(t, func(cx *core.Scope) {
		cx.SetMainView(func(cx *core.Scope) core.Node {
			cx.ShowAlert(core.AlertConfig{Title: "Main?", Actions: []core.AlertAction{{Label: "Yes"}}}, func(int) { answers = append(answers, "main") })
			return core.Text{Content: "main"}
		})
		desktop.OpenWindow(cx, desktop.WindowOptions{Width: 200, Height: 100}, func(cx *core.Scope) core.Node {
			cx.ShowAlert(core.AlertConfig{Title: "Other?", Actions: []core.AlertAction{{Label: "Yes"}}}, func(int) { answers = append(answers, "other") })
			return core.Text{Content: "other"}
		})
	}, Environment(core.Environment{OS: core.OSLinux}), PendingAlerts())
	alerts := app.Alerts()
	if len(alerts) != 2 || alerts[0].ID != alerts[1].ID {
		t.Fatalf("alerts %+v, want two numbered alike", alerts)
	}
	want := map[string]string{"Main?": "main", "Other?": "other"}[alerts[1].Title]
	app.answerAt(1, 0)
	if len(answers) != 1 || answers[0] != want {
		t.Fatalf("answering %q reached %v", alerts[1].Title, answers)
	}
}
