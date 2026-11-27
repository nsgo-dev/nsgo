//go:build darwin && !ios

package macos

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Run registers AppKit classes once per process, so each run (-count included) needs its own process.
func inQuitTestProcess(t *testing.T) bool {
	t.Helper()
	if os.Getenv("NSGO_TEST_QUIT_PROCESS") == t.Name() {
		t.Setenv("HOME", t.TempDir())
		return true
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), "NSGO_TEST_QUIT_PROCESS="+t.Name(), "GOTRACEBACK=all")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return false
}

// TestTerminateAsksTheAppAndReturnsFromRun sends AppKit's terminate: as Cmd+Q, the Dock and a tray menu do, each from a block on the main queue as a tray menu's Quit is: the first request is canceled, the second deferred while a third terminate: comes and then approved, and Run returns with OnQuit run.
func TestTerminateAsksTheAppAndReturnsFromRun(t *testing.T) {
	if !inQuitTestProcess(t) {
		return
	}
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
			if q.Forced() {
				t.Error("terminate: reported a forced quit")
			}
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

func TestMacOSLastWindowAsksBeforeQuitting(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if !inQuitTestProcess(t) {
		return
	}
	visibleWindow := func() objc.ID {
		for s := range surfaces {
			if s.id != 0 && !s.closed && s.window != 0 && objc.Send[bool](s.window, sel("isVisible")) {
				return s.window
			}
		}
		return 0
	}
	var requestClose func()
	requestClose = func() {
		apple.OnMain(func() {
			if w := visibleWindow(); w != 0 {
				w.Send(sel("performClose:"), objc.ID(0))
				return
			}
			time.AfterFunc(5*time.Millisecond, requestClose)
		})
	}
	var closing, requested, quits atomic.Int32
	app := core.AppRoot(func(cx *core.Scope) {
		w := cx.OpenWindow(core.WindowOptions{Title: "Last window", Width: 200, Height: 100}, func(*core.Scope) core.Node {
			return core.Text{Content: "Last window"}
		})
		w.OnClosing(cx, func(c *core.WindowClosing) {
			if closing.Add(1) == 1 {
				c.Cancel()
				requestClose()
			}
		})
		cx.OnQuitRequest(func(q *core.QuitRequest) {
			n := requested.Add(1)
			if q.Forced() || closing.Load() != n+1 || w.Closed() {
				t.Error("quit request did not follow an approved close on an open window")
			}
			if n == 1 {
				q.Cancel()
				requestClose()
				return
			}
			done := q.Defer()
			apple.OnMain(func() {
				if visibleWindow() == 0 {
					t.Error("window disappeared before the deferred answer")
				}
				done(true)
			})
		})
		cx.OnQuit(func(context.Context) { quits.Add(1) })
		requestClose()
	})
	watchdog := time.AfterFunc(10*time.Second, func() { panic("Run did not return after approving the last window's close") })
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Last window quit", HideDock: true}) })
	if got := closing.Load(); got != 3 {
		t.Errorf("OnClosing ran %d times, want 3", got)
	}
	if got := requested.Load(); got != 2 {
		t.Errorf("OnQuitRequest ran %d times, want 2", got)
	}
	if got := quits.Load(); got != 1 {
		t.Errorf("OnQuit ran %d times, want 1", got)
	}
}

func TestSessionQuitAppleEventReasons(t *testing.T) {
	onMain(func() {
		loadAppKit()
		class := objc.ID(apple.Class("NSAppleEventDescriptor"))
		for _, tc := range []struct {
			code   uint32
			forced bool
		}{
			{0, false},          // ordinary quit
			{0x71616c6c, false}, // 'qall', kAEQuitAll
			{0x6c6f676f, true},  // 'logo', kAELogOut
			{0x726c676f, true},  // 'rlgo', kAEReallyLogOut
			{0x72727374, true},  // 'rrst', kAEShowRestartDialog
			{0x72657374, true},  // 'rest', kAERestart
			{0x7273646e, true},  // 'rsdn', kAEShowShutdownDialog
			{0x73687574, true},  // 'shut', kAEShutDown
		} {
			event := class.Send(sel("appleEventWithEventClass:eventID:targetDescriptor:returnID:transactionID:"),
				quitEventClass, quitEventID, objc.ID(0), int16(-1), int32(0))
			if tc.code != 0 {
				event.Send(sel("setParamDescriptor:forKeyword:"), class.Send(sel("descriptorWithEnumCode:"), tc.code), quitReasonKey)
			}
			if got := isForcedQuitEvent(event); got != tc.forced {
				t.Errorf("quit reason %#x: forced = %v, want %v", tc.code, got, tc.forced)
			}
		}
	})
}

func TestSessionQuitAppleEventReturnsSuccess(t *testing.T) {
	const readyEnv = "NSGO_TEST_SESSION_QUIT_READY"
	if ready := os.Getenv(readyEnv); ready != "" {
		t.Setenv("HOME", filepath.Dir(ready))
		var requested, cleaned atomic.Bool
		app := core.AppRoot(func(cx *core.Scope) {
			cx.OnQuitRequest(func(q *core.QuitRequest) {
				if !q.Forced() {
					q.Defer()
					if err := os.WriteFile(ready+".user", nil, 0o600); err != nil {
						t.Error(err)
					}
					return
				}
				q.Cancel()
				q.Defer()(false)
				requested.Store(!q.Canceled())
			})
			cx.OnQuit(func(context.Context) {
				cleaned.Store(requested.Load())
				if err := os.WriteFile(ready+".cleaned", nil, 0o600); err != nil {
					t.Error(err)
				}
			})
			apple.OnMain(func() {
				if err := os.WriteFile(ready, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			})
		})
		onMain(func() { Run(app, Options{Title: "Session Quit", HideDock: true}) })
		if !cleaned.Load() {
			t.Fatal("OnQuit did not run after the forced request")
		}
		return
	}

	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), readyEnv+"="+ready)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	defer func() { cancel(); cmd.Wait() }()
	waitFor := func(path string) {
		t.Helper()
		for {
			if _, err := os.Stat(path); err == nil {
				return
			}
			select {
			case <-ctx.Done():
				cmd.Wait()
				t.Fatalf("test app did not write %s: %s", filepath.Base(path), out.String())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	send := func(reason uint32) int32 {
		var code int32
		onMain(func() {
			loadAppKit()
			class := objc.ID(apple.Class("NSAppleEventDescriptor"))
			target := class.Send(sel("descriptorWithProcessIdentifier:"), int32(cmd.Process.Pid))
			event := class.Send(sel("appleEventWithEventClass:eventID:targetDescriptor:returnID:transactionID:"),
				quitEventClass, quitEventID, target, int16(-1), int32(0))
			if reason != 0 {
				event.Send(sel("setParamDescriptor:forKeyword:"), class.Send(sel("descriptorWithEnumCode:"), reason), quitReasonKey)
			}
			const waitReply, doNotPrompt = 3, 0x00020000
			var sendError objc.ID
			reply := event.Send(sel("sendEventWithOptions:timeout:error:"), uint(waitReply|doNotPrompt), float64(5), unsafe.Pointer(&sendError))
			if reply == 0 {
				t.Fatalf("sending quit Apple event: %s (%d)", apple.GoString(sendError.Send(sel("localizedDescription"))), objc.Send[int](sendError, sel("code")))
			}
			code = objc.Send[int32](reply.Send(sel("paramDescriptorForKeyword:"), errorNumberKey), sel("int32Value"))
		})
		return code
	}
	waitFor(ready)
	if code := send(0); code != -128 { // userCanceledErr
		t.Fatalf("ordinary quit Apple event reply error = %d, want -128", code)
	}
	waitFor(ready + ".user")
	if code := send(0x726c676f); code != 0 { // 'rlgo', kAEReallyLogOut
		t.Fatalf("session quit Apple event reply error = %d", code)
	}
	if _, err := os.Stat(ready + ".cleaned"); err != nil {
		t.Fatalf("the system received its reply before OnQuit ran: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("test app failed: %v\n%s", err, out.String())
	}
}
