package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// A malformed Android pipe can end before TestMain starts the host. The marked error must end the run immediately, without waiting for logcat (a stream that stays open) or accepting an earlier run's failure.
func TestReadMarkedRunReportsRuntimeFailureBeforeStart(t *testing.T) {
	for _, started := range []bool{false, true} {
		t.Run(map[bool]string{false: "before start", true: "after start"}[started], func(t *testing.T) {
			r, w := io.Pipe()
			t.Cleanup(func() { r.Close(); w.Close() })
			deadline := time.AfterFunc(5*time.Second, func() { w.Close() })
			defer deadline.Stop()
			go func() {
				io.WriteString(w, "nsgo-test-error old an earlier failure\n")
				if started {
					io.WriteString(w, "nsgo-test-start t1\n")
				}
				io.WriteString(w, "nsgo-test-error t1 NSGo pipe: stdout is reserved; call nsgotest.Main from TestMain\r\n")
			}()
			code, err := readMarkedRun(r, "t1", &strings.Builder{}, func() error {
				t.Error("waited for the device log to end")
				return errors.New("ended")
			})
			if code != -1 || err == nil || err.Error() != "NSGo pipe: stdout is reserved; call nsgotest.Main from TestMain" {
				t.Fatalf("code %d, err %v", code, err)
			}
		})
	}
}

func TestReadMarkedRunKeepsThisRunsOutput(t *testing.T) {
	lines := strings.Join([]string{
		"Launched application with dev.nsgo.app bundle identifier.",
		"nsgo-test-start old",
		"from an earlier run",
		"nsgo-test-start t1\r",
		"=== RUN   TestA",
		"nsgo-screenshot ~YQ 1/1 iVBOR",
		"--- PASS: TestA (0.01s)",
		"nsgo-test-exit t1 3",
		"after the mark",
	}, "\n")
	var out strings.Builder
	code, err := readMarkedRun(strings.NewReader(lines), "t1", &out, func() error { return errors.New("ended") })
	if err != nil || code != 3 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if want := "=== RUN   TestA\nnsgo-screenshot ~YQ 1/1 iVBOR\n--- PASS: TestA (0.01s)\n"; out.String() != want {
		t.Errorf("out %q, want %q", out.String(), want)
	}
}

func TestReadMarkedRunWithoutAnExitMark(t *testing.T) {
	ended := errors.New("ended")
	if _, err := readMarkedRun(strings.NewReader("nsgo-test-start t1\n=== RUN TestA\n"), "t1", &strings.Builder{}, func() error { return ended }); err != ended {
		t.Errorf("err %v", err)
	}
	if _, err := readMarkedRun(strings.NewReader("nsgo-test-start t1\nnsgo-test-exit t1 x\n"), "t1", &strings.Builder{}, func() error { return ended }); err == nil || err == ended {
		t.Errorf("a mark without a code: %v", err)
	}
}

// Exercise the launcher process itself: simctl can fail after a fast app has already reported its result, or exit either way after the app has crashed.
func TestSimulatorTestReadsTheAppsExitCode(t *testing.T) {
	for _, tc := range []struct {
		name, body, output string
		wantCode           int
		wantError          string
	}{
		{name: "fast app before launch failure", body: `printf 'nsgo-test-start %s\n--- PASS: TestFast (0.00s)\nnsgo-test-exit %s 0\n' "$SIMCTL_CHILD_NSGO_TEST_RUN" "$SIMCTL_CHILD_NSGO_TEST_RUN"
echo 'The process did launch, but has since exited or crashed' >&2
exit 3
`, output: "--- PASS: TestFast", wantCode: 0},
		{name: "failed app with successful launcher", body: `printf 'nsgo-test-start %s\n--- FAIL: TestFast (0.00s)\nnsgo-test-exit %s 7\n' "$SIMCTL_CHILD_NSGO_TEST_RUN" "$SIMCTL_CHILD_NSGO_TEST_RUN"
`, output: "--- FAIL: TestFast", wantCode: 7},
		{name: "crashed app with successful launcher", body: `printf 'nsgo-test-start %s\n=== RUN   TestCrash\n' "$SIMCTL_CHILD_NSGO_TEST_RUN"
echo 'panic: simulator crash fixture' >&2
`, output: "panic: simulator crash fixture", wantCode: -1, wantError: "without its exit code"},
		{name: "crashed app with failing launcher", body: `printf 'nsgo-test-start %s\n=== RUN   TestCrash\n' "$SIMCTL_CHILD_NSGO_TEST_RUN"
echo 'panic: simulator crash fixture' >&2
exit 2
`, output: "panic: simulator crash fixture", wantCode: -1, wantError: "without its exit code"},
		{name: "launch failure", body: "echo 'Simulator device failed to launch' >&2\nexit 4\n", output: "failed to launch", wantCode: -1, wantError: "launch simulator app"},
		{name: "host never started", body: "echo 'ok without a host'\n", output: "ok without a host", wantCode: -1, wantError: "nsgotest.Main"},
		{name: "completed app with lingering launcher", body: `printf 'nsgo-test-start %s\n--- PASS: TestFast (0.00s)\nnsgo-test-exit %s 0\n' "$SIMCTL_CHILD_NSGO_TEST_RUN" "$SIMCTL_CHILD_NSGO_TEST_RUN"
exec sleep 30
`, output: "--- PASS: TestFast", wantCode: 0},
		{name: "runtime error with lingering launcher", body: `printf 'nsgo-test-error %s runtime fixture failure\n' "$SIMCTL_CHILD_NSGO_TEST_RUN"
exec sleep 30
`, wantCode: -1, wantError: "runtime fixture failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkXcrun(t, "case \"$2\" in terminate) exit 0;; esac\n"+tc.body)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			var output strings.Builder
			code, err := (simulatorTest{udid: "simulator", id: "dev.nsgo.test", timeout: 2 * time.Second}).run(ctx, []string{"-test.run=^TestFast$"}, nil, &output)
			if code != tc.wantCode || tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("code=%d err=%v, want code=%d error=%q; output:\n%s", code, err, tc.wantCode, tc.wantError, output.String())
			}
			if !strings.Contains(output.String(), tc.output) {
				t.Fatalf("output lost %q: %s", tc.output, output.String())
			}
			// A lingering launcher must end with the marked result, not hold the run until the test's deadline.
			if ctx.Err() != nil {
				t.Fatal("the run waited for the launcher until its deadline")
			}
		})
	}
}

func TestSimulatorTestSpawnsAPlainProcess(t *testing.T) {
	checkXcrun(t, `[ "$1 $2 $3 $4" = "simctl spawn simulator /app/Plain" ] || exit 9
[ "$SIMCTL_CHILD_KEY" = value ] || exit 8
echo "--- FAIL: TestFast ($5)"
exit 3
`)
	var output strings.Builder
	code, err := (simulatorTest{udid: "simulator", id: "dev.nsgo.test", binary: "/app/Plain"}).run(t.Context(), []string{"-test.run=^TestFast$"}, []string{"KEY=value"}, &output)
	if code != 3 || err != nil || !strings.Contains(output.String(), "--- FAIL: TestFast (-test.run=^TestFast$)") {
		t.Fatalf("code=%d err=%v output=%q", code, err, output.String())
	}
}

func TestSimulatorTestTimeout(t *testing.T) {
	checkXcrun(t, "case \"$2\" in terminate) exit 0;; esac\necho 'waiting for SpringBoard' >&2\nexec sleep 30\n")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var output strings.Builder
	code, err := (simulatorTest{udid: "simulator", id: "dev.nsgo.test", timeout: time.Second}).run(ctx, nil, nil, &output)
	if code != -1 || err == nil || !strings.Contains(err.Error(), "1s") {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if !strings.Contains(output.String(), "waiting for SpringBoard") {
		t.Fatalf("timeout lost launcher diagnostics: %s", output.String())
	}
}

func TestSimulatorStopIsBounded(t *testing.T) {
	checkXcrun(t, "exec sleep 30\n")
	start := time.Now()
	(simulatorTest{udid: "simulator", id: "dev.nsgo.test"}).stop()
	if elapsed := time.Since(start); elapsed > 7*time.Second {
		t.Fatalf("simulator cleanup took %v after its deadline", elapsed)
	}
}
