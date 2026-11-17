package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetAllSkippedSubtests(t *testing.T) {
	const output = `=== RUN   TestParent
=== RUN   TestParent/a
    skip_test.go:10: requires native <host>
=== RUN   TestParent/b
=== RUN   TestParent/b/nested
    skip_test.go:12: no device
--- PASS: TestParent (0.3s)
    --- SKIP: TestParent/a (0.1s)
    --- PASS: TestParent/b (0.2s)
        --- SKIP: TestParent/b/nested (0.2s)
PASS
`
	for _, depth := range []int{0, 2} {
		got := parseTestOutput("TestParent", depth, output, 0, nil)
		if got.Status != "skip" {
			t.Errorf("depth %d: status = %s, want skip", depth, got.Status)
		}
		var out bytes.Buffer
		r := &testRun{results: []*testResult{got}, out: &out}
		r.print(got)
		report := filepath.Join(t.TempDir(), "tests.xml")
		if err := r.finish(testOptions{junit: report}, nil); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "0 passed, 0 failed, 1 skipped") || !strings.Contains(out.String(), "- TestParent") {
			t.Errorf("reported skipped parent: %s", out.String())
		}
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "no device") {
			t.Errorf("skip reason missing from JUnit: %s", data)
		}
	}
	for _, c := range []struct {
		output, want string
		code         int
	}{
		{"--- PASS: TestParent (0.1s)\n--- SKIP: TestParent/a (0.1s)\n--- PASS: TestParent/b (0.1s)\n", "pass", 0},
		{"--- FAIL: TestParent (0.1s)\n--- SKIP: TestParent/a (0.1s)\n", "fail", 1},
		{"--- PASS: TestParent (0.1s)\n--- PASS: TestParent/a (0.1s)\n--- PASS: TestParent (0.1s)\n--- SKIP: TestParent/a (0.1s)\n", "pass", 0},
		{"--- PASS: TestParent (0.1s)\n--- SKIP: TestParent/a (0.1s)\n", "fail", 1},
	} {
		if got := parseTestOutput("TestParent", 0, c.output, c.code, nil); got.Status != c.want {
			t.Errorf("%q (exit %d): status %s, want %s", c.output, c.code, got.Status, c.want)
		}
	}
}

func TestHeadlessAllSkippedSubtests(t *testing.T) {
	var out bytes.Buffer
	r := &testRun{out: &out, running: map[string]*testResult{}, packages: map[string]*testResult{}}
	for _, ev := range []testEvent{
		{Action: "run", Package: "p", Test: "TestParent"},
		{Action: "run", Package: "p", Test: "TestParent/a"},
		{Action: "output", Package: "p", Test: "TestParent/a", Output: "    skip_test.go:10: requires a device\n"},
		{Action: "skip", Package: "p", Test: "TestParent/a"},
		{Action: "pass", Package: "p", Test: "TestParent"},
	} {
		r.event(ev)
	}
	if err := r.finish(testOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0 passed, 0 failed, 1 skipped") || !strings.Contains(out.String(), "- TestParent ") {
		t.Errorf("headless skipped group: %s", out.String())
	}
}

// Exercise the real Go event stream and verbose target output, including nested skips, mixed outcomes, and the skip messages that CI reads.
func TestSkippedResultsEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs test fixtures")
	}
	dir := t.TempDir()
	for name, text := range map[string]string{
		"go.mod": "module example.com/skips\n\ngo 1.26\n",
		"skip_test.go": `package skips
import "testing"
func TestAll(t *testing.T) {
    t.Run("a", func(t *testing.T) { t.Skip("requires native <host>") })
    t.Run("b", func(t *testing.T) { t.Run("nested", func(t *testing.T) { t.Skip("no device") }) })
}
func TestMixed(t *testing.T) {
    t.Run("skip", func(t *testing.T) { t.Skip("not here") })
    t.Run("pass", func(t *testing.T) {})
}
func TestDirect(t *testing.T) { t.Skip("direct reason") }
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"", "web"} {
		t.Run(map[string]string{"": "headless", "web": "web"}[target], func(t *testing.T) {
			if target == "web" {
				if _, err := findBrowser(); err != nil {
					t.Skip(err)
				}
			}
			p, err := loadForTest(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(t.TempDir(), "tests.xml")
			if err := runTests(t.Context(), p, testOptions{target: target, junit: report, env: []string{"GOWORK=off"}}); err != nil {
				t.Fatal(err)
			}
			suites := readJUnitFixture(t, report)
			var found int
			for _, s := range suites.Suites {
				for _, c := range s.Cases {
					if strings.Contains(c.Name, "/") {
						continue
					}
					found++
					if wantSkip := c.Name != "TestMixed"; (c.Skipped != nil) != wantSkip || c.Failure != nil {
						t.Errorf("%s: skipped=%v failure=%v", c.Name, c.Skipped, c.Failure)
					}
				}
			}
			data, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			if found != 3 || !strings.Contains(string(data), "no device") || !strings.Contains(string(data), "direct reason") {
				t.Errorf("JUnit skipped cases and messages: %s", data)
			}
		})
	}
}
