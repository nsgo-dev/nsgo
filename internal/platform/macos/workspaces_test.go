//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestWindowWorkspacesOutliveTheLevel keeps a window on every space and over full screen spaces whatever its level and constraints become, and gives a window on one space its level's and constraints' behavior back.
func TestWindowWorkspacesOutliveTheLevel(t *testing.T) {
	onMain(func() { windowWorkspacesOutliveTheLevel(t) })
}

func windowWorkspacesOutliveTheLevel(t *testing.T) {
	loadAppKit()
	win := objc.ID(apple.Class("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"),
		apple.Rect(0, 0, 200, 100), uint(styleTitled|styleResizable), uint(2), false)
	win.Send(sel("setReleasedWhenClosed:"), false)
	defer win.Send(selRelease)
	s := &surface{window: win}
	const managed = joinAllSpaces | moveToActiveSpace | stationary | ignoresCycle | fullScreenAuxiliary | fullScreenNone
	behavior := func() uint { return objc.Send[uint](win, sel("collectionBehavior")) & managed }
	level := func() int { return objc.Send[int](win, sel("level")) }

	everywhere := uint(joinAllSpaces | fullScreenAuxiliary | ignoresCycle)
	s.setWorkspaces(driver.WindowWorkspacesAll)
	if got := behavior(); got != everywhere {
		t.Fatalf("on every workspace the behavior is %#x; want %#x", got, everywhere)
	}
	s.setLevel(driver.WindowLevelFloating)
	if got := behavior(); got != everywhere || level() != levelFloating {
		t.Fatalf("floated, the behavior is %#x at level %d; want %#x at %d", got, level(), everywhere, levelFloating)
	}
	s.constraints = driver.WindowConstraints{NoMaximize: true}
	s.applySpaces()
	if got := behavior(); got != everywhere {
		t.Fatalf("with NoMaximize the behavior is %#x; want %#x, an auxiliary window never entering full screen", got, everywhere)
	}
	s.setLevel(driver.WindowLevelDesktop)
	if got, want := behavior(), uint(joinAllSpaces|stationary|ignoresCycle|fullScreenNone); got != want {
		t.Fatalf("at the desktop's level the behavior is %#x; want %#x", got, want)
	}
	s.setLevel(driver.WindowLevelNormal)
	if got := behavior(); got != everywhere {
		t.Fatalf("back at the normal level the behavior is %#x; want %#x", got, everywhere)
	}
	s.setWorkspaces(driver.WindowWorkspacesCurrent)
	if got := behavior(); got != fullScreenNone {
		t.Fatalf("on one workspace the behavior is %#x; want only NoMaximize's %#x", got, uint(fullScreenNone))
	}
}
