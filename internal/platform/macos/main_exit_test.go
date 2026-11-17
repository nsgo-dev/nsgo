//go:build darwin && !ios

package macos

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

func TestOnMainExitPropagation(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"fatal", "failnow", "skip", "panic", "goexit", "fatal-then-panic"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable,
				"-test.run=^TestOnMain(Exit|ExitAgain|Survivor)Child$", "-test.v", "-test.timeout=5s")
			cmd.Env = append(os.Environ(), "NSGO_MAIN_EXIT_CASE="+mode)
			output, err := cmd.CombinedOutput()
			text := string(output)
			if mode == "skip" {
				if err != nil || !strings.Contains(text, "SKIP: TestOnMainExitChild") {
					t.Fatalf("skip was not reported: %v\n%s", err, text)
				}
			} else if err == nil || !strings.Contains(text, "main-thread "+mode) {
				t.Fatalf("failure was not reported: %v\n%s", err, text)
			}
			if strings.Contains(text, "ON_MAIN_CONTINUED") || strings.Contains(text, "test timed out") || ctx.Err() != nil {
				t.Fatalf("the test continued after an exit or hung: %v\n%s", err, text)
			}
			if mode != "panic" && mode != "goexit" && mode != "fatal-then-panic" && !strings.Contains(text, "PASS: TestOnMainSurvivorChild") {
				t.Fatalf("later test did not run on the main thread: %v\n%s", err, text)
			}
			if (mode == "panic" || mode == "fatal-then-panic") && !strings.Contains(text, "exitOnMain.func1") {
				t.Fatalf("panic lost the originating main-thread stack:\n%s", text)
			}
			// A panic on the main thread after an earlier exit still reaches its own test: the earlier exit left the loop serving from a deferred call.
			if mode == "fatal-then-panic" && (!strings.Contains(text, "--- FAIL: TestOnMainExitChild") || !strings.Contains(text, "main-thread fatal-then-panic again")) {
				t.Fatalf("the panic after an exit was not reported:\n%s", text)
			}
		})
	}
}

func TestOnMainExitChild(t *testing.T)      { exitOnMain(t, false) }
func TestOnMainExitAgainChild(t *testing.T) { exitOnMain(t, true) }

func exitOnMain(t *testing.T, again bool) {
	mode := os.Getenv("NSGO_MAIN_EXIT_CASE")
	if mode == "" {
		t.Skip("subprocess helper")
	}
	onMain(func() {
		switch mode {
		case "fatal-then-panic":
			if again {
				panic("main-thread fatal-then-panic again")
			}
			t.Fatal("main-thread fatal-then-panic")
		case "fatal":
			t.Fatal("main-thread fatal")
		case "failnow":
			t.Log("main-thread failnow")
			t.FailNow()
		case "skip":
			t.Skip("main-thread skip")
		case "panic":
			panic("main-thread panic")
		case "goexit":
			t.Log("main-thread goexit")
			runtime.Goexit()
		}
	})
	t.Error("ON_MAIN_CONTINUED")
}

func TestOnMainSurvivorChild(t *testing.T) {
	if os.Getenv("NSGO_MAIN_EXIT_CASE") == "" {
		t.Skip("subprocess helper")
	}
	var main bool
	onMain(func() {
		main = objc.Send[bool](objc.ID(objc.GetClass("NSThread")), objc.RegisterName("isMainThread"))
	})
	if !main {
		t.Fatal("later native work ran on a different OS thread")
	}
}
