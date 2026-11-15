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
	if !p.bare || p.Dir != dir || p.ID != generatedID(filepath.Base(dir)) || !p.idDefault {
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
