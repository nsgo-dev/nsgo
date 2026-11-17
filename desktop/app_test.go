package desktop_test

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestLaunchedAtLoginChoosesInitialView(t *testing.T) {
	for _, login := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "login"}[login], func(t *testing.T) {
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				// Read once when the app first runs, as a tray app decides whether to open its welcome window.
				if desktop.LaunchedAtLogin(cx) {
					return core.Text{Content: "Background"}
				}
				return core.Text{Content: "Welcome"}
			}, nsgotest.Environment(core.Environment{LaunchedAtLogin: login}))
			if login {
				app.Find("text:Background")
			} else {
				app.Find("text:Welcome")
			}
		})
	}
}

func TestWindowLevelAndPlacementFollowTheEnvironment(t *testing.T) {
	var cx *core.Scope
	app := nsgotest.Start(t, func(c *core.Scope) core.Node {
		cx = c
		text := func(s string) func(*core.Scope) core.Node {
			return func(*core.Scope) core.Node { return core.Text{Content: s} }
		}
		return core.View{Children: []core.Node{
			core.Show{When: func() bool { return desktop.CanSetWindowLevel(c) }, Then: text("Floats"), Else: text("Stacked")},
			core.Show{When: func() bool { return desktop.CanPlaceWindows(c) }, Then: text("Placed"), Else: text("Unplaced")},
		}}
	}, nsgotest.Environment(core.Environment{OS: core.OSLinux, Idiom: core.IdiomDesktop}))
	// As on Wayland: the app tells, and the level still goes to the platform, which ignores it.
	app.Find("text:Stacked")
	app.Find("text:Unplaced")
	cx.Post(func() { desktop.SetWindowLevel(cx, desktop.WindowLevelFloating) })
	app.Settle()
	if n := len(app.Window); n == 0 || app.Window[n-1] != (core.WindowCommand{Action: core.WindowSetLevel, Level: desktop.WindowLevelFloating}) {
		t.Fatalf("window commands = %+v", app.Window)
	}
	// As on X11, then macOS and Windows.
	app.SetEnvironment(core.Environment{OS: core.OSLinux, Idiom: core.IdiomDesktop, CanSetWindowLevel: true})
	app.Find("text:Floats")
	app.Find("text:Unplaced")
	app.SetEnvironment(core.Environment{OS: core.OSWindows, Idiom: core.IdiomDesktop, CanSetWindowLevel: true, CanPlaceWindows: true})
	app.Find("text:Floats")
	app.Find("text:Placed")
}

// TestWindowWorkspacesFollowTheEnvironment checks that an app can tell whether SetWindowWorkspaces takes effect, as on Windows and Wayland, where it does not and the request still goes to the platform, and on macOS and X11, where it does.
func TestWindowWorkspacesFollowTheEnvironment(t *testing.T) {
	var cx *core.Scope
	app := nsgotest.Start(t, func(c *core.Scope) core.Node {
		cx = c
		text := func(s string) func(*core.Scope) core.Node {
			return func(*core.Scope) core.Node { return core.Text{Content: s} }
		}
		return core.Show{When: func() bool { return desktop.CanSetWindowWorkspaces(c) }, Then: text("Everywhere"), Else: text("Here")}
	}, nsgotest.Environment(core.Environment{OS: core.OSWindows, Idiom: core.IdiomDesktop}))
	app.Find("text:Here")
	cx.Post(func() { desktop.SetWindowWorkspaces(cx, desktop.WindowWorkspacesAll) })
	app.Settle()
	if n := len(app.Window); n == 0 || app.Window[n-1] != (core.WindowCommand{Action: core.WindowSetWorkspaces, Workspaces: desktop.WindowWorkspacesAll}) {
		t.Fatalf("window commands = %+v", app.Window)
	}
	app.SetEnvironment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, CanSetWindowWorkspaces: true})
	app.Find("text:Everywhere")
}

func TestFrontmostFollowsTheAppInFront(t *testing.T) {
	editor := desktop.FrontmostApp{ID: "com.example.editor", Name: "Editor", PID: 41}
	player := desktop.FrontmostApp{ID: "com.example.player", Name: "Player", PID: 42, FullScreen: true}
	var seen []desktop.FrontmostApp
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.Effect(func() { seen = append(seen, desktop.Frontmost(cx)) })
		return core.View{}
	}, nsgotest.Environment(core.Environment{FrontmostApp: editor}))
	app.SetEnvironment(core.Environment{FrontmostApp: player})
	// Another change to the environment leaves the app in front alone.
	app.SetEnvironment(core.Environment{FrontmostApp: player, Dark: true})
	app.SetEnvironment(core.Environment{})
	if want := []desktop.FrontmostApp{editor, player, {}}; !slices.Equal(seen, want) {
		t.Fatalf("frontmost apps %v, want %v", seen, want)
	}
}

// refuseAll is the Access of code that may reach nothing.
type refuseAll struct{}

func (refuseAll) Allows(core.AccessRequest) bool { return false }
func (refuseAll) WebHosts() []string             { return nil }

// onlyFrontmost allows the frontmost app alone.
type onlyFrontmost struct{}

func (onlyFrontmost) Allows(req core.AccessRequest) bool {
	return req.Permission == core.AccessFrontmostApp
}
func (onlyFrontmost) WebHosts() []string { return nil }

func TestFrontmostIsHeldToAccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		access core.Access
		want   string
	}{
		{"refused", refuseAll{}, "text:none"},
		{"granted", onlyFrontmost{}, "text:Editor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				cx.ProvideAccess(tc.access)
				if name := desktop.Frontmost(cx).Name; name != "" {
					return core.Text{Content: name}
				}
				return core.Text{Content: "none"}
			}, nsgotest.Environment(core.Environment{FrontmostApp: desktop.FrontmostApp{ID: "com.example.editor", Name: "Editor"}}))
			app.Find(tc.want)
		})
	}
}
