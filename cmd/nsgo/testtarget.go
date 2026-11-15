package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// testTimeout bounds one test's run on a target.
const testTimeout = 3 * time.Minute

// targetRunner runs one test of the built package on a target: the test
// binary with args, its environment holding env, writing what it prints
// to out. It returns the process's exit code.
type targetRunner struct {
	run   func(ctx context.Context, args, env []string, out io.Writer) (int, error)
	close func()
}

// runTargetTests runs the app's tests on opts.target, each test in a
// process of its own, as the platform shows one app at a time: the test
// binary, built with the backend's test host, is the target's package.
func runTargetTests(ctx context.Context, p *project, opts testOptions) error {
	// A package that builds only for its target, as a backend's, is
	// listed as the target sees it.
	here, err := buildsHere(ctx, p)
	if err != nil {
		return err
	}
	seen := opts.target
	if here {
		seen = ""
	}
	names, err := listTests(ctx, p, seen, opts.run)
	if err != nil {
		return err
	}
	if p.steps != nil {
		names = []string{stepsTest}
	}
	if len(names) == 0 {
		plain("no tests match")
		return nil
	}
	p.testBinary = true
	progress := begin("building the tests for %s", opts.target)
	runner, err := targetFor(ctx, p, opts)
	if err != nil {
		progress.fail()
		return err
	}
	progress.done(fmt.Sprintf("built the tests for %s", opts.target), "")
	defer runner.close()
	shots := opts.screenshots
	if shots == "" {
		shots = filepath.Join(p.Dir, ".nsgo", "test", "screenshots", opts.target)
	} else if !filepath.IsAbs(shots) {
		shots, err = filepath.Abs(shots)
		if err != nil {
			return err
		}
	}
	r := &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, trace: opts.trace, out: os.Stderr}
	savedPaths := map[string]bool{}
	// A package that links no platform, as a library's, runs headless in
	// the target's runtime: the web's engine, say.
	env := slices.Clone(opts.env)
	if hosted, err := linksPlatform(ctx, p, seen); err != nil {
		return err
	} else if hosted {
		env = append(env, "NSGO_TEST_TARGET="+opts.target)
	}
	if opts.trace {
		env = append(env, "NSGO_TEST_TRACE=1")
	}
	if opts.fps {
		env = append(env, "NSGO_TEST_FPS=1")
	}
	for _, name := range names {
		var out bytes.Buffer
		w := io.MultiWriter(&out, traceWriter{r.out, opts.trace, opts.fps})
		tctx, cancel := context.WithCancel(ctx)
		if opts.timeout > 0 {
			tctx, cancel = context.WithTimeout(ctx, opts.timeout)
		}
		started := time.Now()
		code, err := runner.run(tctx, targetTestArgs(name, opts.run, opts.timeout), env, w)
		timedOut := errors.Is(tctx.Err(), context.DeadlineExceeded)
		cancel()
		if timedOut {
			code, err = -1, timeoutError(opts.timeout)
		} else if ctx.Err() != nil {
			return ctx.Err()
		}
		t, saved := targetResult(name, out.String(), code, err, shots, savedPaths)
		if t.Elapsed == 0 {
			t.Elapsed = time.Since(started)
		}
		t.Package = p.ID
		r.results = append(r.results, t)
		r.print(t)
		for _, s := range saved {
			fmt.Fprintf(r.out, "%s  → %s\n", margin, rel(s))
		}
	}
	return r.finish(opts, nil)
}

// traceWriter passes a test's trace lines on as they come: the batches
// with trace, the performance monitor's readings with fps, and the marks
// of what they measure (nsgotest's Mark) always, so a run without the
// monitor can be timed by them too.
type traceWriter struct {
	w          io.Writer
	trace, fps bool
}

func (t traceWriter) Write(b []byte) (int, error) {
	for _, line := range strings.SplitAfter(string(b), "\n") {
		switch {
		case t.trace && (strings.HasPrefix(line, "──") || strings.HasPrefix(line, "   ")):
			io.WriteString(t.w, line)
		case strings.Contains(line, "nsgo: perf ") && (t.fps || strings.Contains(line, "nsgo: perf mark ")):
			io.WriteString(t.w, margin+line[strings.Index(line, "nsgo: perf ")+len("nsgo: "):])
		}
	}
	return len(b), nil
}

// linksPlatform reports whether the package's tests link NSGo's root
// package, which links the platform's test host: as target builds them,
// or this machine for "".
func linksPlatform(ctx context.Context, p *project, target string) (bool, error) {
	out, err := goListFor(ctx, p, target, "-test", "-deps", "-f", "{{.ImportPath}}")
	if err != nil {
		return false, err
	}
	return slices.Contains(strings.Fields(string(out)), "github.com/nsgo-dev/nsgo"), nil
}

// goListFor runs go list with args on the package as target's test build
// sees it, or this machine's for "".
func goListFor(ctx context.Context, p *project, target string, args ...string) ([]byte, error) {
	var env []string
	if target != "" {
		var err error
		if env, err = vetEnv(ctx, target); err != nil {
			return nil, err
		}
	}
	tags, err := goTags(ctx, p, env, append([]string{"nsgotest"}, targetTags(target)...)...)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "go", append(append(append([]string{"list"}, tagsArgs(tags)...), args...), ".")...)
	cmd.Dir = p.Dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w\n%s", err, stderr.Bytes())
	}
	return out, nil
}

// listTests lists the package's tests that run matches, as go test -list
// does on this machine, or from its sources as target sees them.
func listTests(ctx context.Context, p *project, target, run string) ([]string, error) {
	if run == "" {
		run = "."
	}
	if target != "" {
		top, _ := splitTestRun(run)
		return listTestSources(ctx, p, target, top)
	}
	tags, err := goTags(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	args := append([]string{"test", "-list", run}, tagsArgs(tags)...)
	// The TSX tests are in a file the tool adds.
	if tsxTest, err := p.tsxTestFile(); err != nil {
		return nil, err
	} else if tsxTest != "" {
		ov, err := p.overlay(map[string]string{"zz_nsgo_tsx_test.go": tsxTest})
		if err != nil {
			return nil, err
		}
		args = append(args, "-overlay", ov)
	}
	cmd := exec.CommandContext(ctx, "go", append(args, ".")...)
	cmd.Dir = p.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go test -list: %w\n%s%s", err, out, stderr.Bytes())
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Test") || strings.HasPrefix(line, "Example") {
			names = append(names, strings.TrimSpace(line))
		}
	}
	return names, nil
}

// splitTestRun returns the top-level part of go test's hierarchical -run
// expression and the remaining subtest expression. Slashes inside character
// classes or parentheses are part of the regexp, as they are in testing.
func splitTestRun(run string) (top, sub string) {
	classes, groups := 0, 0
	for i := 0; i < len(run); i++ {
		switch run[i] {
		case '[':
			classes++
		case ']':
			if classes > 0 {
				classes--
			}
		case '(':
			if classes == 0 {
				groups++
			}
		case ')':
			if classes == 0 && groups > 0 {
				groups--
			}
		case '\\':
			i++
		case '/':
			if classes == 0 && groups == 0 {
				return run[:i], run[i+1:]
			}
		}
	}
	return run, ""
}

// targetTestArgs runs exactly one top-level test while retaining go test's
// hierarchical subtest matching when -run contains a slash.
func targetTestArgs(name, run string, timeout time.Duration) []string {
	_, sub := splitTestRun(run)
	pattern := "^" + regexp.QuoteMeta(name) + "$"
	if sub != "" {
		pattern += "/" + sub
	}
	args := []string{"-test.run", pattern, "-test.v"}
	if timeout > 0 {
		budget := timeout - time.Second
		if budget <= 0 {
			budget = timeout
		}
		args = append(args, "-test.timeout", budget.String())
	}
	return args
}

func timeoutError(timeout time.Duration) error {
	return fmt.Errorf("the test took over %v", timeout)
}

func testContextError(ctx context.Context, timeout time.Duration) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return timeoutError(timeout)
	}
	return ctx.Err()
}

// buildsHere reports whether the package has Go files for this machine.
func buildsHere(ctx context.Context, p *project) (bool, error) {
	tags, err := goTags(ctx, p, nil, "nsgotest")
	if err != nil {
		return false, err
	}
	cmd := exec.CommandContext(ctx, "go", append(append([]string{"list", "-e"}, tagsArgs(tags)...), "-f", "{{if or .GoFiles .TestGoFiles .XTestGoFiles}}yes{{end}}", ".")...)
	cmd.Dir = p.Dir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("go list: %w", err)
	}
	return strings.TrimSpace(string(out)) == "yes", nil
}

// listTestSources lists the top-level tests that run matches in the
// package's test files for target.
func listTestSources(ctx context.Context, p *project, target, run string) ([]string, error) {
	re, err := regexp.Compile(run)
	if err != nil {
		return nil, fmt.Errorf("-run: %w", err)
	}
	out, err := goListFor(ctx, p, target, "-f", "{{range .TestGoFiles}}{{$.Dir}}/{{.}}\n{{end}}{{range .XTestGoFiles}}{{$.Dir}}/{{.}}\n{{end}}")
	if err != nil {
		return nil, err
	}
	var names []string
	fset := token.NewFileSet()
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !isTestName(fn.Name.Name) || !re.MatchString(fn.Name.Name) {
				continue
			}
			names = append(names, fn.Name.Name)
		}
	}
	return names, nil
}

// isTestName reports whether name names a test, as go test reads it:
// Test, then no lower-case letter.
func isTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return rest == "" || !unicode.IsLower(r)
}

// targetResult turns one test process's output into its result and saves the screenshots it carries in dir, returning the files written.
func targetResult(name, out string, code int, runErr error, dir string, seen map[string]bool) (*testResult, []string) {
	text, saved, saveErr := saveScreenshots(out, dir, seen)
	t := parseTestOutput(name, text, code, runErr)
	// A screenshot the tool cannot save, as one whose path an earlier one took, fails its test, and the run goes on.
	if saveErr != nil {
		t.failFor(saveErr.Error())
	}
	return t, saved
}

// testLine matches go test -v's result line of a top-level test.
var testLine = regexp.MustCompile(`^--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+)s\)`)

// parseTestOutput turns one test process's output into its result; a
// process that printed no result for it crashed or timed out.
func parseTestOutput(name, out string, code int, runErr error) *testResult {
	t := &testResult{Name: name, Output: out}
	matched := ""
	for _, line := range strings.Split(out, "\n") {
		if m := testLine.FindStringSubmatch(line); m != nil && (m[2] == name || strings.HasPrefix(m[2], name+"/")) && len(m[2]) > len(matched) {
			matched = m[2]
			t.Status = strings.ToLower(m[1])
			s, _ := strconv.ParseFloat(m[3], 64)
			t.Elapsed = time.Duration(s * float64(time.Second))
		}
	}
	if matched != "" {
		t.Name = matched
	}
	if t.Status == "" || t.Status == "pass" && code != 0 {
		why := fmt.Sprintf("the test process exited with %d", code)
		if runErr != nil {
			why = runErr.Error()
		}
		// A process that crashed before its result keeps the crash's first line as the message.
		t.failFor(why)
	}
	return t
}

// screenshotLine is how nsgotest hands the tool a screenshot: its encoded
// name, then the PNG in base64, in chunks a device's log keeps whole.
var screenshotLine = regexp.MustCompile(`nsgo-screenshot (\S+) (\d+)/(\d+) (\S*)`)

// saveScreenshots saves the screenshots out carries in dir and returns out without them, the files written and an error for each shot whose path is invalid, taken before or cannot be written, which it skips.
func saveScreenshots(out, dir string, seen map[string]bool) (string, []string, error) {
	var kept strings.Builder
	// Each Screenshot call is a shot of its own, so a name taken twice in one process is a duplicate rather than one image's chunks overwritten.
	type shot struct {
		name  string
		parts []string
	}
	var shots []*shot
	current := map[string]*shot{}
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 256<<20)
	for sc.Scan() {
		line := sc.Text()
		m := screenshotLine.FindStringSubmatch(line)
		if m == nil {
			kept.WriteString(line + "\n")
			continue
		}
		name := m[1]
		if strings.HasPrefix(name, "~") {
			decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(name, "~"))
			if err != nil {
				kept.WriteString("screenshot: invalid name encoding\n")
				continue
			}
			name = string(decoded)
		}
		i, _ := strconv.Atoi(m[2])
		n, _ := strconv.Atoi(m[3])
		// A first chunk starts a new shot of the name.
		s := current[name]
		if s == nil || i == 1 {
			s = &shot{name: name, parts: make([]string, n)}
			current[name] = s
			shots = append(shots, s)
		}
		if i >= 1 && i <= len(s.parts) {
			s.parts[i-1] = m[4]
		}
	}
	var saved []string
	var errs []error
	for _, s := range shots {
		name := s.name
		data, err := base64.StdEncoding.DecodeString(strings.Join(s.parts, ""))
		if err != nil {
			kept.WriteString("screenshot " + name + ": " + err.Error() + "\n")
			continue
		}
		// A name that cleans to "." names dir itself, which .png would turn into a file beside it.
		if name == "" || filepath.Clean(name) == "." || filepath.IsAbs(name) || screenshotPathHasParent(name) {
			errs = append(errs, fmt.Errorf("screenshot %q: name must be a relative file path without ..", name))
			continue
		}
		path := filepath.Join(dir, name)
		if filepath.Ext(path) != ".png" {
			path += ".png"
		}
		path = filepath.Clean(path)
		if seen[path] || sameFileAsSeen(path, seen) {
			errs = append(errs, fmt.Errorf("screenshot %q resolves to %s more than once", name, path))
			continue
		}
		seen[path] = true
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			errs = append(errs, fmt.Errorf("screenshot %q: create directory: %w", name, err))
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			errs = append(errs, fmt.Errorf("screenshot %q: %w", name, err))
			continue
		}
		saved = append(saved, path)
	}
	return kept.String(), saved, errors.Join(errs...)
}

// sameFileAsSeen reports whether path names the file of a path in seen under another spelling, as "Home.png" and "home.png" do on the case-insensitive volumes macOS and Windows default to.
func sameFileAsSeen(path string, seen map[string]bool) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	for p := range seen {
		if pi, err := os.Stat(p); err == nil && os.SameFile(fi, pi) {
			return true
		}
	}
	return false
}

func screenshotPathHasParent(name string) bool {
	for _, part := range strings.FieldsFunc(filepath.ToSlash(name), func(r rune) bool { return r == '/' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

// targetFor builds the tests as opts.target's package and returns how to
// run them there.
func targetFor(ctx context.Context, p *project, opts testOptions) (targetRunner, error) {
	dir := p.cache("test", opts.target)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return targetRunner{}, err
	}
	local := func(bin string) targetRunner {
		return targetRunner{run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			return runLocal(ctx, bin, args, env, out)
		}, close: func() {}}
	}
	switch opts.target {
	case "macos":
		if runtime.GOOS != "darwin" {
			return targetRunner{}, fmt.Errorf("the macos target runs on a Mac")
		}
		app, err := buildMacApp(ctx, p, dir, runtime.GOARCH)
		if err != nil {
			return targetRunner{}, err
		}
		return local(filepath.Join(app, "Contents", "MacOS", p.Name)), nil
	case "windows":
		if runtime.GOOS != "windows" {
			return targetRunner{}, fmt.Errorf("the windows target runs on Windows")
		}
		if err := buildWindows(ctx, p, dir, runtime.GOARCH); err != nil {
			return targetRunner{}, err
		}
		return local(filepath.Join(dir, p.Name+".exe")), nil
	case "linux":
		if runtime.GOOS != "linux" {
			return targetRunner{}, fmt.Errorf("the linux target runs on Linux")
		}
		if err := buildLinux(ctx, p, dir, runtime.GOARCH); err != nil {
			return targetRunner{}, err
		}
		return local(filepath.Join(dir, p.Name)), nil
	case "web":
		return webTestRunner(ctx, p, dir, opts)
	case "ios_simulator":
		return iosTestRunner(ctx, p, iOS, dir, opts)
	case "tvos_simulator":
		return iosTestRunner(ctx, p, tvOS, dir, opts)
	case "watchos_simulator":
		return iosTestRunner(ctx, p, watchOS, dir, opts)
	case "android":
		return androidTestRunner(ctx, p, dir, opts)
	}
	return targetRunner{}, fmt.Errorf("unknown test target %q; use macos, windows, linux, web, ios_simulator, tvos_simulator, watchos_simulator or android", opts.target)
}

// runLocal runs bin on this machine.
func runLocal(ctx context.Context, bin string, args, env []string, out io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = filepath.Dir(bin)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), nil
	}
	return 0, err
}
