package engine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

type faultPlatform struct {
	recorder
	broken bool // only the engine goroutine writes or reads it
}

func (p *faultPlatform) Prepare(text *driver.AttributedText) driver.PreparedText {
	if p.broken {
		panic("injected text fault")
	}
	return p.ApproxTextEngine.Prepare(text)
}

// A subprocess keeps this regression safe even before a fault boundary exists, and proves an engine with CheckCommits still terminates the test process.
func TestEngineFaultIsolation(t *testing.T) {
	if mode := os.Getenv("NSGO_TEST_ENGINE_FAULT"); mode != "" {
		runEngineFault(t, mode)
		return
	}
	for _, mode := range []string{"initial", "commit", "message", "thread", "cleanup", "check"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEngineFaultIsolation$", "-test.v")
			cmd.Env = append(os.Environ(), "NSGO_TEST_ENGINE_FAULT="+mode)
			out, err := cmd.CombinedOutput()
			if mode == "check" {
				// The fault report alone also prints the panic, so only the runtime's exit status for an unrecovered panic shows that the engine re-panicked.
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(out), "panic: injected text fault") {
					t.Fatalf("CheckCommits did not re-panic: %v\n%s", err, out)
				}
			} else if err != nil {
				t.Fatalf("engine fault escaped isolation: %v\n%s", err, out)
			}
		})
	}
}

func runEngineFault(t *testing.T, mode string) {
	healthy := &recorder{}
	var goodScope *core.Scope
	var goodSignal *core.Signal[string]
	good := Start(healthy, func(cx *core.Scope) core.Node {
		goodScope, goodSignal = cx, core.NewSignal(cx, "alive")
		return core.Text{ContentFunc: goodSignal.Get}
	}, Options{Size: driver.Size{Width: 200, Height: 100}})
	defer good.Close()
	good.Sync()

	p := &faultPlatform{broken: mode == "initial" || mode == "check"}
	errorsSeen := make(chan error, 4)
	var failedScope *core.Scope
	var signal *core.Signal[string]
	var work context.Context
	opts := Options{CheckCommits: mode == "check", Size: driver.Size{Width: 200, Height: 100}, Host: HostOptions{
		Channel: NewHost(), OnError: func(err error) { errorsSeen <- err },
	}}
	if mode != "initial" && mode != "check" && mode != "thread" {
		opts.Windows = &WindowOptions{Width: 200, Height: 100}
	}
	if mode == "thread" {
		opts.Thread = func() { panic("injected thread fault") }
	}
	e := Start(p, func(cx *core.Scope) core.Node {
		failedScope, signal = cx, core.NewSignal(cx, "before")
		work = cx.Context()
		if opts.Windows != nil {
			cx.OpenWindow(WindowOptions{}, func(*core.Scope) core.Node { return core.Text{Content: "second surface"} })
		}
		if mode == "cleanup" {
			cx.Cleanup(func() { panic("injected cleanup fault") })
		}
		return core.Text{ContentFunc: signal.Get}
	}, opts)
	defer e.Close()
	var resized <-chan struct{}
	var attached, queuedAttach *recorder
	if mode != "initial" && mode != "thread" && mode != "check" {
		e.Sync()
		// Attach a second real surface before the fault, and leave another attachment queued behind a failing message during that pass. The shared graph must never commit again on either platform.
		attached = &recorder{}
		p.wait(t, "the second surface to open", func() bool { return len(p.items(driver.ItemOpenSurface)) > 0 })
		cfg := p.items(driver.ItemOpenSurface)[0].Surface
		second := e.Attach(cfg.ID, attached)
		second.Resize(driver.Size{Width: 200, Height: 100}, driver.Geometry{})
		attached.wait(t, "the second surface to commit", func() bool { return len(attached.texts()) > 0 })
		queuedAttach = &recorder{}
		blocked, release := make(chan struct{}), make(chan struct{})
		e.push(message{fn: func() { close(blocked); <-release }})
		waitFault(t, blocked)
		if mode == "message" {
			e.enqueue(message{fn: func() { panic("injected message fault") }})
		} else {
			e.enqueue(message{fn: func() { p.broken = true; signal.Set("after") }})
		}
		resized = e.ResizeAndWait(driver.Size{Width: 220, Height: 100}, driver.Geometry{})
		e.Attach(99, queuedAttach)
		close(release)
	}
	waitFault(t, e.exited)
	if mode == "check" {
		t.Fatal("CheckCommits swallowed its panic")
	}
	var reported *core.PanicError
	for len(errorsSeen) > 0 {
		err := <-errorsSeen
		var candidate *core.PanicError
		if errors.As(err, &candidate) && strings.Contains(candidate.Error(), "injected") &&
			!strings.Contains(candidate.Error(), "panic: injected cleanup fault") {
			reported = candidate
		}
	}
	if reported == nil || !strings.Contains(string(reported.Stack), "engine.(*Engine).run") {
		t.Fatalf("OnError did not receive the internal panic and engine stack: %v", reported)
	}
	if !surfaceDestroyed(&p.recorder) {
		t.Fatal("failed engine's surface survived")
	}
	if attached != nil && !surfaceDestroyed(attached) {
		t.Fatal("pending attachment survived")
	}
	if queuedAttach != nil && !surfaceDestroyed(queuedAttach) {
		t.Fatal("pending attachment survived")
	}
	if work != nil {
		waitFault(t, work.Done())
	}
	if resized != nil {
		waitFault(t, resized)
	}
	// Waiters and late events must all finish without accumulating work.
	waitFault(t, e.ResizeAndWait(driver.Size{}, driver.Geometry{}))
	e.Sync()
	e.Hide()
	if lang := e.Language(); lang != "" {
		t.Fatalf("failed engine language = %q", lang)
	}
	late := &recorder{}
	e.Attach(100, late)
	if !surfaceDestroyed(late) {
		t.Fatal("late attachment survived")
	}
	if failedScope != nil {
		failedScope.Post(func() { t.Error("failed graph ran a post") })
	}
	e.mu.Lock()
	queued := len(e.queue)
	e.mu.Unlock()
	if queued != 0 {
		t.Fatalf("stopped engine retained %d messages", queued)
	}
	goodScope.Post(func() { goodSignal.Set("still alive") })
	good.Sync()
	healthy.wait(t, "the other engine's next commit", func() bool {
		return slices.Contains(slices.Collect(values(healthy.texts())), "still alive")
	})
}

func waitFault(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fault shutdown left a waiter blocked")
	}
}

func surfaceDestroyed(p *recorder) bool {
	return slices.ContainsFunc(p.items(driver.ItemWindow), func(it driver.MountItem) bool {
		return it.Window.Action == driver.WindowDestroy
	})
}

func TestOnPanicHook(t *testing.T) {
	prev := OnPanic
	t.Cleanup(func() { OnPanic = prev })

	got := make(chan string, 1)
	OnPanic = func(value any, stack []byte) {
		if value == "hooked panic" && len(stack) > 0 {
			select {
			case got <- string(stack):
			default:
			}
		}
	}

	p := &recorder{}
	e := Start(p, func(*core.Scope) core.Node { panic("hooked panic") }, Options{
		Size: driver.Size{Width: 100, Height: 100},
	})
	defer e.Close()

	select {
	case stack := <-got:
		if !strings.Contains(stack, "TestOnPanicHook") {
			t.Fatalf("unexpected stack:\n%s", stack)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnPanic was not called for uncaught panic")
	}
}
