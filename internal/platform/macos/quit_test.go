//go:build darwin && !ios

package macos

import (
	"context"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestTerminateAsksTheAppAndReturnsFromRun sends AppKit's terminate: as Cmd+Q, the Dock and a tray menu do, each from a block on the main queue as a tray menu's Quit is: the first request is canceled, the second deferred while a third terminate: comes and then approved, and Run returns with OnQuit run.
func TestTerminateAsksTheAppAndReturnsFromRun(t *testing.T) {
	// Run registers the app's AppKit classes once a process, so each run of the test, -count's included, gets a process of its own.
	if os.Getenv("NSGO_TEST_QUIT_PROCESS") != t.Name() {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_QUIT_PROCESS="+t.Name(), "GOTRACEBACK=all")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	t.Setenv("HOME", t.TempDir())
	var terminate func()
	terminate = func() {
		apple.OnMain(func() {
			// Cancel's response crosses the engine's mount batch. Wait for that response before issuing a new request; otherwise the backend correctly treats it as part of the still-pending one.
			if quitPending {
				time.AfterFunc(time.Millisecond, terminate)
				return
			}
			nsApp().Send(sel("terminate:"), nsApp())
		})
	}
	var requests, quits atomic.Int32
	app := core.AppRoot(func(cx *core.Scope) {
		cx.OnQuitRequest(func(q *core.QuitRequest) {
			switch requests.Add(1) {
			case 1:
				q.Cancel()
				terminate()
			case 2:
				done := q.Defer()
				apple.OnMain(func() {
					// A second Cmd+Q while the app asks waits for its answer.
					nsApp().Send(sel("terminate:"), nsApp())
					go done(true)
				})
			}
		})
		cx.OnQuit(func(context.Context) { quits.Add(1) })
		terminate()
	})
	watchdog := time.AfterFunc(20*time.Second, func() { panic("Run did not return after an approved terminate:") })
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Quit", Width: 200, Height: 100}) })
	if got := requests.Load(); got != 2 {
		t.Errorf("the app heard %d quit requests, want 2", got)
	}
	if got := quits.Load(); got != 1 {
		t.Errorf("OnQuit ran %d times, want 1", got)
	}
}
