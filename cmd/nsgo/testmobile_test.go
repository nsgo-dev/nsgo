package main

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// A malformed Android pipe can end before TestMain starts the host. The
// marked error must end the run immediately, without waiting for logcat
// (a stream that stays open) or accepting an earlier run's failure.
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
