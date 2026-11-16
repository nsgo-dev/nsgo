package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestTargetTeardownEndToEnd runs testdata/teardown's tests on this machine's desktop target, where its TestMain calls nsgotest.MainFunc: each test's process runs the teardown deferred after m.Run once the test is done, and ends with the code MainFunc's function returns, not m.Run's. It opens windows, so it runs only when asked, on a desktop with a GUI session:
//
//	NSGO_TARGET_E2E=1 go tool nsgo test -run TestTargetTeardownEndToEnd ./cmd/nsgo
func TestTargetTeardownEndToEnd(t *testing.T) {
	target := map[string]string{"darwin": "macos", "windows": "windows", "linux": "linux"}[runtime.GOOS]
	if target == "" || os.Getenv("NSGO_TARGET_E2E") == "" {
		t.Skip("set NSGO_TARGET_E2E=1 on a macOS, Windows or Linux desktop with a GUI session: the test opens windows")
	}
	p, err := loadForTest(filepath.Join("..", "..", "testdata", "teardown"), nil)
	if err != nil {
		t.Fatal(err)
	}
	marks := func(env ...string) (string, error) {
		mark := filepath.Join(t.TempDir(), "mark")
		err := runTests(t.Context(), p, testOptions{target: target, env: append(env, "NSGO_TEARDOWN_MARK="+mark)})
		b, _ := os.ReadFile(mark)
		return string(b), err
	}

	got, err := marks()
	if err != nil {
		t.Fatalf("the tests failed: %v", err)
	}
	lines := strings.Fields(got)
	var tests []string
	for i := 0; i+1 < len(lines); i += 2 {
		if lines[i+1] != "teardown" {
			break
		}
		tests = append(tests, lines[i])
	}
	slices.Sort(tests)
	if len(lines) != 4 || !slices.Equal(tests, []string{"TestFirst", "TestSecond"}) {
		t.Fatalf("the processes wrote %q, want each test followed by its process's teardown", got)
	}

	got, err = marks("NSGO_TEARDOWN_CODE=1")
	if err == nil {
		t.Fatal("the tests passed though MainFunc's function returned 3")
	}
	if strings.Count(got, "teardown") != 2 {
		t.Fatalf("the processes wrote %q, want a teardown in each", got)
	}
}
