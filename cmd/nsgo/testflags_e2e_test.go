package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestTargetGoTestFlags exercises a real target executable. This fixture is a library without NSGo's native host: it verifies Go's testing flags and reporting, and needs no window or unlocked desktop.
func TestTargetGoTestFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a target executable")
	}
	target := map[string]string{"darwin": "macos", "linux": "linux", "windows": "windows"}[runtime.GOOS]
	if target == "" {
		t.Skip("needs a desktop target runner")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod": "module flagfixture\n\ngo 1.26\n",
		"flags_test.go": `package fixture
import ("os"; "runtime"; "testing")
var repeats int
func mark(t *testing.T, text string) {
 t.Helper()
 f, err := os.OpenFile(os.Getenv("NSGO_FLAGS_MARK"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
 if err != nil { t.Fatal(err) }; defer f.Close()
 if _, err := f.WriteString(text+"\n"); err != nil { t.Fatal(err) }
}

func TestShort(t *testing.T) {
 if testing.Short() { t.Skip("short was forwarded") }
 t.Fatal("short was silently dropped")
}
func TestRepeat(t *testing.T) {
 repeats++
 if repeats == 1 { t.Skip("the first repetition") }
 t.Run("selected", func(t *testing.T) {
  if runtime.GOMAXPROCS(0) != 2 { t.Fatal("cpu was silently dropped") }
  mark(t, "repeat")
 })
}
func TestFirstFailure(t *testing.T) { t.Fatal("stop here") }
func TestLater(t *testing.T) { mark(t, "later") }
`,
	})
	run := func(pattern string, args []string) (junitSuites, string, error) {
		t.Helper()
		p, err := loadForTest(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		out := t.TempDir()
		mark, report := filepath.Join(out, "mark"), filepath.Join(out, "report.xml")
		err = runTests(t.Context(), p, testOptions{target: target, run: pattern, args: args, junit: report, env: []string{"NSGO_FLAGS_MARK=" + mark, "GOMAXPROCS=1"}})
		var suite junitSuites
		data, readErr := os.ReadFile(report)
		if readErr != nil {
			t.Fatalf("report: %v (run: %v)", readErr, err)
		}
		if e := xml.Unmarshal(data, &suite); e != nil {
			t.Fatal(e)
		}
		marks, _ := os.ReadFile(mark)
		return suite, string(marks), err
	}

	r, marks, err := run("TestShort", []string{"-short"})
	if err != nil || r.Tests != 1 || r.Failures != 0 || r.Suites[0].Skipped != 1 || marks != "" {
		t.Fatalf("-short: %+v, marks=%q, %v", r, marks, err)
	}
	r, marks, err = run("TestRepeat/selected", []string{"-count", "3", "-cpu=2", "-parallel=2", "-fullpath"})
	if err != nil || r.Tests != 1 || r.Failures != 0 || r.Suites[0].Skipped != 0 || marks != "repeat\nrepeat\n" {
		t.Fatalf("-count/-cpu/subtest: %+v, marks=%q, %v", r, marks, err)
	}
	r, marks, err = run("TestFirstFailure|TestLater", []string{"-failfast"})
	if err == nil || r.Tests != 1 || r.Failures != 1 || strings.Contains(marks, "later") {
		t.Fatalf("-failfast: %+v, marks=%q, %v", r, marks, err)
	}
	r, marks, err = run("TestLater", []string{"-count=0"})
	if err != nil || r.Tests != 0 || marks != "" {
		t.Fatalf("-count=0: %+v, marks=%q, %v", r, marks, err)
	}
}

func TestTargetTestProcessIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a target executable")
	}
	target := map[string]string{"darwin": "macos", "linux": "linux", "windows": "windows"}[runtime.GOOS]
	if target == "" {
		t.Skip("needs a desktop target runner")
	}
	p, err := loadForTest(filepath.Join("..", "..", "testdata", "testflags"), nil)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	mark, report := filepath.Join(out, "mark"), filepath.Join(out, "report.xml")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	shots, err := filepath.Rel(cwd, filepath.Join(out, "screenshots"))
	if err != nil {
		t.Fatal(err)
	}
	err = runTests(t.Context(), p, testOptions{target: target, run: "TestIsolated", args: []string{"-count=3", "-cpu=1,2"}, junit: report, screenshots: shots, env: []string{"NSGO_FLAGS_MARK=" + mark}})
	if err != nil {
		t.Fatal(err)
	}
	marks, err := os.ReadFile(mark)
	if err != nil || string(marks) != "1\n1\n1\n2\n2\n2\n" {
		t.Fatalf("native process repetitions: %q, %v", marks, err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var suite junitSuites
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatal(err)
	}
	if suite.Tests != 6 || suite.Failures != 0 || suite.Suites[0].Skipped != 0 {
		t.Fatalf("native repetitions report: %s", data)
	}
	for i := 1; i <= 6; i++ {
		shot := filepath.Join(out, "screenshots", fmt.Sprintf("run-%d", i), "repeated.png")
		if data, err := os.ReadFile(shot); err != nil || string(data) != "shot" {
			t.Errorf("repetition %d screenshot: %q, %v", i, data, err)
		}
	}
}
