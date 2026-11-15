package desktop_test

import (
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
