//go:build darwin && !ios && nsgotest

package macos

import (
	"os"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// The test host: nsgo test -target macos runs each test in a window of
// its own, acting with events posted to the application's queue, so they
// take the path a user's do, its key and mouse monitors included.

func init() { testhost.Register(host{}) }

type host struct{}

func (host) Main(run func() int) {
	nsApp := startApp(Options{Title: "NSGo test"}, nil)
	go func() {
		code := run()
		os.Exit(code)
	}()
	nsApp.Send(sel("run"))
}

func (host) Open(app func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	var s *surface
	test := &testWindow{options: o}
	onMainSync(func() {
		s = openWindow(app, Options{Title: "NSGo test", Width: o.Size.Width, Height: o.Size.Height, Perf: o.FPS}, test)
		// In front, as a user sees it, but without taking the keyboard
		// from the app the user works in.
		s.window.Send(sel("orderFrontRegardless"))
		s.window.Send(sel("makeKeyWindow"))
	})
	return &testSurface{s: s, test: test}, nil
}

// Several implements testhost.Host: a desktop shows several windows.
func (host) Several() bool { return true }
