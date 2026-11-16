package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSplitTestRun(t *testing.T) {
	for _, tc := range []struct {
		pattern, top, sub string
	}{
		{"TestGallery/dark-home", "TestGallery", "dark-home"},
		{"TestGallery/[dark/home]", "TestGallery", "[dark/home]"},
		{"TestGallery/(dark/home)", "TestGallery", "(dark/home)"},
		{"TestGallery", "TestGallery", ""},
	} {
		if top, sub := splitTestRun(tc.pattern); top != tc.top || sub != tc.sub {
			t.Errorf("splitTestRun(%q) = %q, %q; want %q, %q", tc.pattern, top, sub, tc.top, tc.sub)
		}
	}
}

func TestTargetTestArgs(t *testing.T) {
	if got := targetTestArgs("TestGallery", "TestGallery/dark-home", 15*time.Minute); !slices.Equal(got,
		[]string{"-test.run", "^TestGallery$/dark-home", "-test.v", "-test.timeout", "14m59s"}) {
		t.Errorf("targetTestArgs = %q", got)
	}
}

func TestParseTargetSubtestOutput(t *testing.T) {
	got := parseTestOutput("TestGallery", "--- PASS: TestGallery/dark-home (0.2s)\n--- PASS: TestGallery (0.2s)\n", 0, nil)
	if got.Name != "TestGallery/dark-home" || got.Status != "pass" {
		t.Fatalf("result = %+v, want the selected subtest", got)
	}
}

func TestTreeOf(t *testing.T) {
	for in, want := range map[string]string{
		"":              "",
		".":             ".",
		"...":           ".",
		"./...":         ".",
		"./core/...":    filepath.FromSlash("./core"),
		"examples/tsx":  "examples/tsx",
		"/abs/pkg/...":  filepath.FromSlash("/abs/pkg"),
		"/...":          "/",
		"App.tsx":       "App.tsx",
		"core/x...y":    "core/x...y",
		"core/...extra": "core/...extra",
	} {
		if got := treeOf(in); got != want {
			t.Errorf("treeOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPackageArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"./other"}, []string{"./other"}},
		{[]string{"./a", "./b"}, []string{"./a", "./b"}},
		{[]string{"github.com/example/app"}, []string{"github.com/example/app"}},
		{[]string{"-run", "TestOne"}, nil},
		{[]string{"-run=TestOne"}, nil},
		{[]string{"--run", "TestOne"}, nil},
		{[]string{"-count", "1"}, nil},
		{[]string{"-shuffle", "on", "-cpuprofile", "cpu.out", "-exec", "wrapper", "-tags", "x"}, nil},
		{[]string{"-test.count", "2", "-test.cpuprofile", "cpu.out"}, nil},
		// A flag of the test binary's own takes the next argument, as go test assumes.
		{[]string{"-update", "golden"}, nil},
		{[]string{"-update=true", "./b"}, []string{"./b"}},
		{[]string{"-v", "./b"}, []string{"./b"}},
		{[]string{"-race", "-short", "-benchmem", "-test.v", "./b"}, []string{"./b"}},
		{[]string{"-v", "-run", "X", "./b"}, []string{"./b"}},
		{[]string{"-args", "./not-a-package"}, nil},
		{[]string{"--args", "./not-a-package"}, nil},
		{[]string{"-v", "--", "./not-a-package"}, nil},
	} {
		if got := packageArgs(tc.args); !slices.Equal(got, tc.want) {
			t.Errorf("packageArgs(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

// TestRunTestsKeepsTheTreeWithFlags: go test flags after the directory, whose values are not packages, and a test binary's flags still run every package under it, the one in a subdirectory too.
func TestRunTestsKeepsTheTreeWithFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test")
	}
	dir := t.TempDir()
	const update = "import (\n\t\"flag\"\n\t\"os\"\n\t\"testing\"\n)\n\nvar update = flag.String(\"update\", \"\", \"\")\n\n"
	for name, content := range map[string]string{
		"go.mod":          "module treetest\n\ngo 1.26\n",
		"root_test.go":    "package root\n\n" + update + "func TestRoot(t *testing.T) { _ = os.Getenv }\n",
		"sub/sub_test.go": "package sub\n\n" + update + "func TestSub(t *testing.T) {\n\tif err := os.WriteFile(os.Getenv(\"TREE_MARK\"), []byte(*update), 0o644); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := loadForTest(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, args := range [][]string{{"-count", "1"}, {"-count=1", "-update", "golden"}, {"-count=1", "-args", "-update=golden"}} {
		mark := filepath.Join(t.TempDir(), "mark")
		if err := runTests(t.Context(), p, testOptions{args: args, env: []string{"TREE_MARK=" + mark}}); err != nil {
			t.Errorf("%d: runTests with %q: %v", i, args, err)
		}
		if _, err := os.Stat(mark); err != nil {
			t.Errorf("%d: runTests with %q did not run sub's test: %v", i, args, err)
		}
	}
}

func TestRunTestsRejectsASecondPackage(t *testing.T) {
	p, err := loadForTest(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"", "web"} {
		err := runTests(t.Context(), p, testOptions{target: target, args: []string{"-v", "./b"}})
		if err == nil || !strings.Contains(err.Error(), `takes one directory, not "./b"`) {
			t.Errorf("target %q: runTests = %v, want an error naming ./b", target, err)
		}
	}
}

func TestLoadForTestTakesBareTrees(t *testing.T) {
	dir := t.TempDir()
	p, err := loadForTest(filepath.Join(dir, "..."), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !p.bare || p.Dir != dir || p.ID != bareID(filepath.Base(dir)) || !p.idDefault {
		t.Fatalf("loadForTest = %+v, want a bare project with generated ID at %s", p, dir)
	}
	if _, err := loadForTest(filepath.Join(dir, "missing"), nil); err == nil {
		t.Fatal("loadForTest took a missing directory")
	}
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: App\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, err = loadForTest(dir, nil); err != nil || p.bare || p.Name != "App" {
		t.Fatalf("loadForTest = %+v, %v, want the app", p, err)
	}
}

// TestBenchmarkResults feeds the events go test -json prints for
// benchmarks, which end with no event of their own, one result split
// over two lines of output.
func TestBenchmarkResults(t *testing.T) {
	var out bytes.Buffer
	r := &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, out: &out}
	r.read(strings.NewReader(`{"Action":"run","Package":"p","Test":"BenchmarkTick"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"=== RUN   BenchmarkTick\n"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"BenchmarkTick\n"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"BenchmarkTick-10   \t"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"  100\t  77375 ns/op\t  26 allocs/op\n"}
{"Action":"output","Package":"p","Test":"BenchmarkBoard/fixed","Output":"BenchmarkBoard/fixed-10 \t 1\t 291583 ns/op\n"}
{"Action":"output","Package":"p","Output":"PASS\n"}
{"Action":"pass","Package":"p","Elapsed":0.2}
`))
	if err := r.finish(testOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	want := margin + "✓ BenchmarkTick-10 100 77375 ns/op 26 allocs/op\n" +
		margin + "✓ BenchmarkBoard/fixed-10 1 291583 ns/op\n" +
		margin + "2 passed, 0 failed, 0 skipped\n"
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
}

// A process panic can end the package without any closing test event. Keep its last test's output, fail the tests it cut short, a paused parallel one included, and leave another package's tests alone.
func TestInterruptedTestsKeepTheirOutput(t *testing.T) {
	var out bytes.Buffer
	r := &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, out: &out}
	r.read(strings.NewReader(`{"Action":"run","Package":"p","Test":"TestParallel"}
{"Action":"pause","Package":"p","Test":"TestParallel"}
{"Action":"run","Package":"p","Test":"TestCrash"}
{"Action":"run","Package":"q","Test":"TestOther"}
{"Action":"output","Package":"p","Test":"TestCrash","Output":"panic: native callback failed\nstack trace\n"}
{"Action":"output","Package":"p","Output":"FAIL\tp\t0.1s\n"}
{"Action":"fail","Package":"p","Elapsed":0.1}
{"Action":"pass","Package":"q","Test":"TestOther","Elapsed":0.2}
{"Action":"pass","Package":"q","Elapsed":0.2}
`))
	err := r.finish(testOptions{}, nil)
	if err == nil || err.Error() != "2 tests failed" {
		t.Errorf("finish = %v, want the interrupted tests to fail", err)
	}
	for _, want := range []string{"✗ TestCrash", "panic: native callback failed", "stack trace", "✗ TestParallel", "✓ TestOther", "1 passed, 2 failed, 0 skipped"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("console misses %q:\n%s", want, out.String())
		}
	}
	if len(r.running) != 0 {
		t.Errorf("unfinished tests = %v", r.running)
	}
}

// A process that crashes outside a test, in an init function or a benchmark, which ends with no event of its own, shows the crash under its package and is not called a build failure; a package that did not build still is.
func TestProcessCrashOutsideTests(t *testing.T) {
	var out bytes.Buffer
	r := &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, out: &out}
	r.read(strings.NewReader(`{"Action":"output","Package":"i","Output":"panic: init failed\n"}
{"Action":"output","Package":"i","Output":"FAIL\ti\t0.1s\n"}
{"Action":"fail","Package":"i","Elapsed":0.1}
{"Action":"run","Package":"b","Test":"BenchmarkTick"}
{"Action":"output","Package":"b","Test":"BenchmarkTick","Output":"BenchmarkTick-10 \t 100\t 77375 ns/op\n"}
{"Action":"output","Package":"b","Test":"BenchmarkTick","Output":"panic: benchmark failed\n"}
{"Action":"output","Package":"b","Output":"FAIL\tb\t0.1s\n"}
{"Action":"fail","Package":"b","Elapsed":0.1}
`))
	err := r.finish(testOptions{}, nil)
	if err == nil || err.Error() != "a test process failed outside its tests" {
		t.Errorf("finish = %v, want a crash outside the tests", err)
	}
	for _, want := range []string{"✗ i", "panic: init failed", "✗ b", "panic: benchmark failed", "1 passed, 0 failed, 0 skipped"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("console misses %q:\n%s", want, out.String())
		}
	}
	if len(r.running) != 0 {
		t.Errorf("unfinished tests = %v", r.running)
	}

	out.Reset()
	r = &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, out: &out}
	r.read(strings.NewReader(`{"ImportPath":"c [c.test]","Action":"build-output","Output":"c_test.go:5:30: undefined: undefined\n"}
{"ImportPath":"c [c.test]","Action":"build-fail"}
{"Action":"output","Package":"c","Output":"FAIL\tc [build failed]\n"}
{"Action":"fail","Package":"c","Elapsed":0,"FailedBuild":"c [c.test]"}
{"Action":"output","Package":"i","Output":"panic: init failed\n"}
{"Action":"fail","Package":"i","Elapsed":0.1}
`))
	if err := r.finish(testOptions{}, nil); err == nil || err.Error() != "tests did not build" {
		t.Errorf("finish = %v, want the build failure", err)
	}
	if !strings.Contains(out.String(), "undefined: undefined") {
		t.Errorf("console misses the compiler's output:\n%s", out.String())
	}
}

// Exercise the real go test event stream, including a panic on another goroutine during a subtest and a test the timeout ends, and the JUnit artifact CI consumers read.
func TestRunTestsReportsProcessPanic(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a test process that panics")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod": "module example.com/crash\n\ngo 1.26\n",
		"crash_test.go": `package crash
import "testing"
func TestCrash(t *testing.T) {
    t.Run("worker", func(t *testing.T) {
        go func() { panic("nsgo process panic fixture") }()
        <-make(chan struct{})
    })
}
`,
		"slow/slow_test.go": `package slow
import "testing"
func TestSlow(t *testing.T) { <-make(chan struct{}) }
`,
	})
	p, err := loadForTest(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(dir, "report.xml")
	if err := runTests(t.Context(), p, testOptions{junit: report, args: []string{"-count=1", "-timeout=3s"}}); err == nil || err.Error() != "2 tests failed" {
		t.Fatalf("runTests = %v, want failed tests rather than a build failure", err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var suites junitSuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"TestCrash/worker": "panic: nsgo process panic fixture", "TestSlow": "panic: test timed out after 3s"}
	for _, s := range suites.Suites {
		for _, c := range s.Cases {
			if w, ok := want[c.Name]; ok && c.Failure != nil && strings.Contains(c.Failure.Body, w) {
				delete(want, c.Name)
			}
		}
	}
	if len(want) != 0 {
		t.Fatalf("JUnit lost the interrupted tests %v or their panics:\n%s", want, data)
	}
}

func TestTSXTestsSkipOwnTrees(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"app/nsgo.yaml", "app/app.test.tsx", "lib/go.mod", "lib/lib.test.tsx", "testdata/x.test.tsx"} {
		path := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := &project{Dir: dir, bare: true}
	if file, err := p.tsxTestFile(); err != nil || file != "" {
		t.Fatalf("tsxTestFile = %q, %v, want none", file, err)
	}
}

func TestIsTestName(t *testing.T) {
	for name, want := range map[string]bool{
		"Test": true, "TestFeel": true, "Test_x": true, "Test2": true,
		"Testing": false, "testFeel": false, "BenchmarkFeel": false,
	} {
		if got := isTestName(name); got != want {
			t.Errorf("isTestName(%q) = %v", name, got)
		}
	}
}

// TestTargetTestsNeedTheTestHost runs a library package's tests on Android and an iPhone, whose runs only the test host can carry: the tool refuses the package before it builds or looks for a device, with what to add, whether the tests link no host or start none from TestMain.
func TestTargetTestsNeedTheTestHost(t *testing.T) {
	if testing.Short() {
		t.Skip("lists a module's packages")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	// NSGo's own requirements, which go test -list wants listed in a module that imports it, and NSGo itself from this tree.
	requires := strings.Replace(string(mod), "module github.com/nsgo-dev/nsgo", "module example.com/lib", 1)
	requires, _, _ = strings.Cut(requires, "\ntool ")
	requires += "\nrequire github.com/nsgo-dev/nsgo v0.0.0\n\nreplace github.com/nsgo-dev/nsgo => " + root + "\n"
	const test = "func TestChat(t *testing.T) {}\n"
	for _, tc := range []struct {
		name, file, want string
	}{
		{"unhosted", "package lib\n\nimport \"testing\"\n\n" + test, "do not link: import github.com/nsgo-dev/nsgo in a test file"},
		{"no TestMain", "package lib\n\nimport (\n\t\"testing\"\n\n\t_ \"github.com/nsgo-dev/nsgo\"\n)\n\n" + test, "add func TestMain(m *testing.M) { nsgotest.Main(m) }"},
	} {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"go.mod":      requires,
			"go.sum":      string(sum),
			"lib.go":      "package lib\n",
			"lib_test.go": tc.file,
		})
		p, err := loadForTest(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range []string{"android", "ios"} {
			// A device would be looked for, and the tests built, only after the check: none is named, so any other error is the tool going on.
			err := runTests(t.Context(), p, testOptions{target: target, device: "no-such-device"})
			if err == nil || !strings.Contains(err.Error(), "-target "+target+" runs the tests in NSGo's test host") || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s on %s: runTests = %v, want an error containing %q", tc.name, target, err, tc.want)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, ".nsgo", "test")); err == nil {
			t.Errorf("%s: the tool started a build in .nsgo/test", tc.name)
		}
	}
}

func TestTestHostError(t *testing.T) {
	if err := testHostError("android", true, true); err != nil {
		t.Errorf("testHostError(hosted, TestMain) = %v, want nil", err)
	}
	if !needsTestHost("android") || !needsTestHost("ios") || needsTestHost("ios_simulator") || needsTestHost("web") || needsTestHost("macos") {
		t.Error("needsTestHost names other targets than android and ios")
	}
}

func TestListTestSourcesLeavesOutTestMain(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":      "module example.com/lib\n\ngo 1.26\n",
		"lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestMain(m *testing.M) { m.Run() }\n\nfunc TestChat(t *testing.T) {}\n\nfunc helper() {}\n",
	})
	p, err := loadForTest(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	names, err := listTestSources(t.Context(), p, "", ".")
	if err != nil || !slices.Equal(names, []string{"TestChat"}) {
		t.Fatalf("listTestSources = %q, %v; want [TestChat]", names, err)
	}
}

func TestSaveScreenshotsKeepsNamesAndRejectsDuplicates(t *testing.T) {
	dir := t.TempDir()
	encode := func(name string) string { return "~" + base64.RawURLEncoding.EncodeToString([]byte(name)) }
	line := func(name string) string { return "nsgo-screenshot " + encode(name) + " 1/1 iVBORw0KGgo=" }
	seen := map[string]bool{}
	if _, saved, err := saveScreenshots(line("en/my library")+"\n"+line("ja/my library")+"\n", dir, seen); err != nil {
		t.Fatal(err)
	} else if len(saved) != 2 {
		t.Fatalf("saved %v, want two screenshots", saved)
	}
	for _, name := range []string{"en/my library.png", "ja/my library.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("screenshot %s: %v", name, err)
		}
	}
	if _, saved, err := saveScreenshots(line("en/my library")+"\n"+line("after")+"\n", dir, seen); err == nil {
		t.Fatal("duplicate screenshot path succeeded")
	} else if len(saved) != 1 || filepath.Base(saved[0]) != "after.png" {
		t.Fatalf("saved %v beside a duplicate, want after.png", saved)
	}
	// Two shots of one name in one process, the second larger, fail without corrupting the first.
	two := "nsgo-screenshot " + encode("twice") + " 1/2 AAAA\nnsgo-screenshot " + encode("twice") + " 2/2 BBBB\n"
	if _, saved, err := saveScreenshots(line("twice")+"\n"+two, dir, map[string]bool{}); err == nil {
		t.Fatal("a name shot twice in one process succeeded")
	} else if len(saved) != 1 {
		t.Fatalf("saved %v, want the first shot", saved)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "twice.png")); err != nil || len(b) != 8 {
		t.Fatalf("twice.png holds %d bytes (%v), want the first shot's 8", len(b), err)
	}
	if _, _, err := saveScreenshots(line("../outside")+"\n", dir, map[string]bool{}); err == nil {
		t.Fatal("parent screenshot path succeeded")
	}
	for _, name := range []string{".", "./"} {
		if _, _, err := saveScreenshots(line(name)+"\n", dir, map[string]bool{}); err == nil {
			t.Errorf("screenshot %q succeeded", name)
		}
	}
	if _, err := os.Stat(dir + ".png"); err == nil {
		t.Error("screenshot \".\" wrote a file beside the directory")
	}
	// A path an earlier screenshot made a directory cannot be written.
	if _, saved, err := saveScreenshots(line("shot.png/a")+"\n"+line("shot.png")+"\n", dir, map[string]bool{}); err == nil {
		t.Fatal("a screenshot over a directory succeeded")
	} else if len(saved) != 1 {
		t.Fatalf("saved %v, want only shot.png/a.png", saved)
	}
	// Names that differ only in case are one file where the volume ignores case.
	seen = map[string]bool{}
	if _, _, err := saveScreenshots(line("en/Home")+"\n", dir, seen); err != nil {
		t.Fatal(err)
	}
	_, folds := os.Stat(filepath.Join(dir, "en", "home.png"))
	_, _, err := saveScreenshots(line("en/home")+"\n", dir, seen)
	if folds == nil && err == nil {
		t.Error("en/home overwrote en/Home on a case-insensitive volume")
	} else if folds != nil && err != nil {
		t.Errorf("en/home beside en/Home on a case-sensitive volume: %v", err)
	}
}

// A passing test the tool fails, for a screenshot it cannot save or a process that exits badly, reports the tool's reason as its JUnit message and prints none of go test's closing lines.
func TestToolFailureIsTheMessage(t *testing.T) {
	dir := t.TempDir()
	shot := "nsgo-screenshot ~" + base64.RawURLEncoding.EncodeToString([]byte("home")) + " 1/1 iVBORw0KGgo=\n"
	passed := "=== RUN   TestShot\n    shot_test.go:9: took it\n" + shot + shot + "--- PASS: TestShot (0.01s)\nPASS\n"
	dup, _ := targetResult("TestShot", passed, 0, nil, dir, map[string]bool{})
	exited, _ := targetResult("TestExit", "=== RUN   TestExit\n--- PASS: TestExit (0.01s)\nPASS\n", 1, nil, dir, map[string]bool{})
	for _, c := range []struct {
		res  *testResult
		want string
	}{
		{dup, `screenshot "home" resolves to`},
		{exited, "the test process exited with 1"},
	} {
		if c.res.Status != "fail" {
			t.Fatalf("%s: status %q, want fail", c.res.Name, c.res.Status)
		}
		c.res.Package = "example.com/app"
		report := filepath.Join(dir, c.res.Name+".xml")
		if err := writeJUnit(report, []*testResult{c.res}, nil); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatal(err)
		}
		var suites junitSuites
		if err := xml.Unmarshal(data, &suites); err != nil {
			t.Fatal(err)
		}
		if got := suites.Suites[0].Cases[0].Failure.Message; !strings.HasPrefix(got, c.want) {
			t.Errorf("%s: JUnit message %q, want %q…", c.res.Name, got, c.want)
		}
		var console bytes.Buffer
		r := &testRun{out: &console}
		r.print(c.res)
		for _, line := range strings.Split(console.String(), "\n") {
			if strings.TrimSpace(line) == "PASS" {
				t.Errorf("%s: console prints go test's PASS line:\n%s", c.res.Name, console.String())
			}
		}
		if !strings.Contains(console.String(), c.want) {
			t.Errorf("%s: console lacks %q:\n%s", c.res.Name, c.want, console.String())
		}
	}
}
