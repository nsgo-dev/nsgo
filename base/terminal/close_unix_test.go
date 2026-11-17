//go:build darwin || linux

package terminal

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestViewCleanupWithInheritedHangupIgnored(t *testing.T) {
	if os.Getenv("NSGO_TEST_TERMINAL_CLOSE") != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", `trap '' HUP; exec "$@"`, "sh",
			os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_TERMINAL_CLOSE=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("view cleanup with inherited SIGHUP ignored: %v\n%s", err, out)
		}
		return
	}
	if !signal.Ignored(syscall.SIGHUP) {
		t.Fatal("helper did not inherit an ignored SIGHUP")
	}
	exited := make(chan int, 1)
	term := newTerminal(t, Options{
		Command: []string{"/bin/sh", "-c", "printf 'ready\\n'; exec sleep 30"},
		OnExit:  func(code int) { exited <- code },
	})
	var shown *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shown = core.NewSignal(cx, true)
		return core.Show{When: shown.Get, Then: func(cx *core.Scope) core.Node {
			// The owner closes its terminal when its view is removed, as
			// the gallery does; a terminal can also outlive a view.
			cx.Cleanup(func() { term.Close() })
			return View{Terminal: term, TestID: "terminal", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}}
		}}
	}, nsgotest.Size(640, 400))
	app.WaitUntil(5*time.Second, func() bool { return strings.Contains(term.Text(), "ready") })
	term.mu.Lock()
	proc := term.proc
	term.mu.Unlock()
	app.Post(func() { shown.Set(false) })
	app.WaitForGone("id:terminal", time.Second)
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the program outlived its view's cleanup")
	}
	if code, err := proc.Wait(); code != 128+int(syscall.SIGTERM) || err != nil {
		t.Fatalf("shell exit = %d, %v; want SIGTERM", code, err)
	}
	select {
	case code := <-exited:
		if code != 128+int(syscall.SIGTERM) {
			t.Fatalf("OnExit = %d, want SIGTERM", code)
		}
	case <-time.After(time.Second):
		t.Fatal("OnExit was not called")
	}
}
