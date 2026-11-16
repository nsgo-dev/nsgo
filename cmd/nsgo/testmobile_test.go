package main

import (
	"errors"
	"strings"
	"testing"
)

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
