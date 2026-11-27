package main

import (
	"flag"
	projectcfg "github.com/nsgo-dev/nsgo/project"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestGoFlagTags(t *testing.T) {
	for _, c := range []struct {
		flags string
		want  []string
	}{
		{"", nil},
		{"-mod=mod -trimpath", nil},
		{"-mod=mod -tags=demo_lite,netgo -trimpath", []string{"demo_lite", "netgo"}},
		{"--tags=demo_lite", []string{"demo_lite"}},
		// The go command applies GOFLAGS in order, so a later -tags replaces an earlier one.
		{"-tags=a -tags=b,c", []string{"b", "c"}},
		{"-tags=", nil},
	} {
		if got := goFlagTags(c.flags); !slices.Equal(got, c.want) {
			t.Errorf("goFlagTags(%q) = %v, want %v", c.flags, got, c.want)
		}
	}
}

// TestGoFlagTagsMatchTheGoCommand: goFlagTags reads GOFLAGS as the go command does.
func TestGoFlagTagsMatchTheGoCommand(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("needs the go command")
	}
	for _, flags := range []string{"-tags=a,b", "--tags=a", "-tags=a -mod=mod -tags=b,c"} {
		t.Setenv("GOFLAGS", flags)
		cmd := exec.CommandContext(t.Context(), "go", "list", "-f", "{{join context.BuildTags \",\"}}", "fmt")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list with GOFLAGS=%s: %v", flags, err)
		}
		if got, want := strings.Join(goFlagTags(flags), ","), strings.TrimSpace(string(out)); got != want {
			t.Errorf("GOFLAGS=%s: goFlagTags = %s, the go command's = %s", flags, got, want)
		}
	}
}

func TestGoTagsMergeEnvironmentAndProject(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod -tags=demo_lite,netgo")
	p := &project{Config: projectcfg.Config{Dir: t.TempDir()}}
	p.Build.Tags = []string{"demo_lite", "timetzdata"}
	tags, err := goTags(t.Context(), p, nil, "nsgodev", "timetzdata")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"demo_lite", "netgo", "timetzdata", "nsgodev"}; !slices.Equal(tags, want) {
		t.Fatalf("goTags = %v, want %v", tags, want)
	}
	// What go env -w wrote counts too.
	t.Setenv("GOFLAGS", "")
	env := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(env, []byte("GOFLAGS=-tags=written\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", env)
	if tags, err := goTags(t.Context(), p, nil); err != nil || !slices.Equal(tags, []string{"written", "demo_lite", "timetzdata"}) {
		t.Fatalf("with go env -w GOFLAGS, goTags = %v, %v", tags, err)
	}
	// Without tags anywhere, a go command gets no -tags.
	if err := os.WriteFile(env, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if tags, err := goTags(t.Context(), &project{Config: projectcfg.Config{Dir: t.TempDir()}}, nil); err != nil || tagsArgs(tags) != nil {
		t.Fatalf("without tags, goTags = %v, %v", tags, err)
	}
}

func TestTargetTags(t *testing.T) {
	for target, want := range map[string][]string{
		"harmonyos":          {"nsgoharmony"},
		"watchos":            {"nsgowatch"},
		"watchos_simulator":  {"nsgowatch"},
		"miniprogram_wechat": miniprogramTags,
		"ios":                nil,
		"macos_arm64":        nil,
		"web":                nil,
	} {
		if got := targetTags(target); !slices.Equal(got, want) {
			t.Errorf("targetTags(%q) = %v, want %v", target, got, want)
		}
	}
	// vetEnv leaves the tags to targetTags, so the app's GOFLAGS stay whole.
	for _, target := range []string{"web", "miniprogram_wechat", "macos_arm64", "android_arm64"} {
		env, err := vetEnv(t.Context(), target)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "GOFLAGS=") }) {
			t.Errorf("vetEnv(%q) = %v sets GOFLAGS", target, env)
		}
	}
}

func TestBuildTagsRejectsInvalid(t *testing.T) {
	for _, key := range []string{"tags", "release_tags", "release_tag"} {
		for _, tags := range []string{"[a,b]", "[\"a,b\"]", "[\"a b\"]", "[\"\"]", "[\"-x\"]"} {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"nsgo.yaml": "name: Test\nbuild:\n  " + key + ": " + tags + "\n"})
			_, err := load(dir)
			switch {
			case key == "release_tag":
				if err == nil || !strings.Contains(err.Error(), "field release_tag not found") {
					t.Errorf("unknown field %s: %v", key, err)
				}
			case tags == "[a,b]":
				if err != nil {
					t.Errorf("%s %s: %v", key, tags, err)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), "build."+key) {
					t.Errorf("%s %s: err = %v, want a build.%s error", key, tags, err, key)
				}
			}
		}
	}
}

// tagsApp writes an app whose main package compiles only with demo_lite, which also links app/extra, and returns its project with build.tags set.
func tagsApp(t *testing.T) *project {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":         "module app\n\ngo 1.26\n",
		"main.go":        "package main\n\nfunc main() { println(flavor) }\n",
		"lite.go":        "//go:build demo_lite\n\npackage main\n\nimport _ \"app/extra\"\n\nconst flavor = \"lite\"\n",
		"full.go":        "//go:build !demo_lite\n\npackage main\n\nconst flavor = missing\n",
		"flavor_test.go": "//go:build demo_lite\n\npackage main\n\nimport \"testing\"\n\nfunc TestFlavor(t *testing.T) {}\n",
		"extra/extra.go": "package extra\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := &project{Config: projectcfg.Config{Dir: dir, Name: "Tags", ID: "com.example.tags", Version: "1.0.0"}}
	p.Build.Tags = []string{"demo_lite"}
	return p
}

// TestBuildTagsReachEveryGoCommand: the app's build.tags reach the go commands the tool runs, with GOFLAGS's tags and the tool's own added rather than replacing them, and GOFLAGS's other flags kept.
func TestBuildTagsReachEveryGoCommand(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("needs the go command")
	}
	t.Setenv("GOFLAGS", "-tags=from_env -mod=mod")
	p := tagsApp(t)
	ctx := t.Context()

	p.devClient = true
	cmd, done, err := goBuildCommand(ctx, p, filepath.Join(t.TempDir(), "app"), nil, buildExtra{tags: []string{"nsgowatch"}})
	if err != nil {
		t.Fatal(err)
	}
	done()
	if i := slices.Index(cmd.Args, "-tags"); i < 0 || cmd.Args[i+1] != "from_env,demo_lite,nsgowatch,nsgodev" {
		t.Errorf("go build's arguments %q, want -tags from_env,demo_lite,nsgowatch,nsgodev", cmd.Args)
	}
	p.devClient = false

	if err := goToolWithTags(ctx, p, nil, nil, "vet", "."); err != nil {
		t.Errorf("vet with build.tags: %v", err)
	}
	if err := goToolWithTags(ctx, p, []string{"GOOS=js", "GOARCH=wasm"}, targetTags("miniprogram_wechat"), "vet", "."); err != nil {
		t.Errorf("vet for a mini program with build.tags: %v", err)
	}

	dir, err := linkedDir(ctx, p, nil, "app/extra")
	if err != nil {
		t.Fatal(err)
	}
	// go list may resolve the temporary directory's symlinks, such as
	// /var to /private/var on macOS. Compare the directories themselves.
	got, statErr := os.Stat(dir)
	want, wantErr := os.Stat(filepath.Join(p.Dir, "extra"))
	if statErr != nil || wantErr != nil || !os.SameFile(got, want) {
		t.Errorf("linkedDir = %q, stat errors %v, %v; want the extra package's directory", dir, statErr, wantErr)
	}
	pkgs, err := p.appPackages(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(pkgs, func(g goPackage) bool { return g.importPath == "app/extra" }) {
		t.Errorf("appPackages leaves out app/extra, which build.tags links: %v", pkgs)
	}

	out, err := goListFor(ctx, p, "macos_arm64", "-deps", "-f", "{{.ImportPath}}")
	if err != nil || !slices.Contains(strings.Fields(string(out)), "app/extra") {
		t.Errorf("goListFor for macos_arm64 = %q, %v; want app/extra listed", out, err)
	}
	names, err := listTests(ctx, p, "", "")
	if err != nil || !slices.Equal(names, []string{"TestFlavor"}) {
		t.Errorf("listTests = %v, %v; want TestFlavor", names, err)
	}

	// Without build.tags or GOFLAGS's tag, the app's main package does not compile.
	t.Setenv("GOFLAGS", "-mod=mod")
	p.Build.Tags = nil
	if err := goToolWithTags(ctx, p, nil, nil, "vet", "."); err == nil {
		t.Error("vet without the tag passed; the fixture does not depend on it")
	}
}

func TestReleaseTagsSelectCompiledFiles(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod -tags=from_env")
	target := tuiTarget(runtime.GOOS, runtime.GOARCH)
	if target == "" {
		t.Skip("needs a native TUI target")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"nsgo.yaml": "name: Tags\nbuild:\n  tags: [project_tag]\n  release_tags: [release]\n",
		"go.mod": "module releasefixture\n\ngo 1.26\n",
		"main.go": "//go:build project_tag && from_env\n\npackage main\nfunc main() { println(flavor) }\n",
		"local.go": "//go:build !release\n\npackage main\nconst flavor = \"local\"\n",
		"release.go": "//go:build release\n\npackage main\nconst flavor = \"release\"\n",
		"main_test.go": "package main\nimport \"testing\"\nfunc TestLocal(t *testing.T) { if flavor != \"local\" { t.Fatal(flavor) } }\n",
	})
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	checkBinary := func(t *testing.T, bin, want string) {
		t.Helper()
		out, err := exec.CommandContext(t.Context(), bin).CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != want {
			t.Fatalf("run %s = %q, %v; want %q", bin, out, err, want)
		}
	}
	for _, tc := range []struct {
		name, want string
		args []string
	}{
		{name: "plain", want: "local"},
		{name: "release", want: "release", args: []string{"-release"}},
		{name: "store", want: "release", args: []string{"-channel", "store"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := *p
			fs := flag.NewFlagSet("build", flag.ContinueOnError)
			f := bindBuildFlags(fs)
			if err := fs.Parse(append([]string{"-target", target}, tc.args...)); err != nil {
				t.Fatal(err)
			}
			f.apply(&q)
			if err := buildAll(t.Context(), &q); err != nil {
				t.Fatal(err)
			}
			checkBinary(t, tuiBinary(&q, q.packageDir(target), target), tc.want)
		})
	}
	// Even a store app's dev -release compiles the local source variant.
	p.Updates.Channel = "store"
	q := *p
	q.Release, q.devClient = true, true
	devDir := t.TempDir()
	if err := buildTUI(t.Context(), &q, devDir, target); err != nil {
		t.Fatal(err)
	}
	checkBinary(t, tuiBinary(&q, devDir, target), "local")
	if err := runTests(t.Context(), p, testOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseTagsVetBothVariants(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=from_env -mod=mod")
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"nsgo.yaml": "name: Tags\nbuild:\n  tags: [project_tag]\n  release_tags: [release]\n",
		"go.mod": "module releasefixture\n\ngo 1.26\n",
		"main.go": "package main\nfunc main() {}\n",
		"release.go": "//go:build release && from_env && project_tag && (!js || nsgoharmony)\n\npackage main\nimport \"fmt\"\nfunc bad() { fmt.Printf(\"%d\", \"not an integer\") }\n",
	})
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"", "harmonyos"} {
		if err := check(t.Context(), p, target, 0); err == nil ||
			!strings.Contains(err.Error(), "release tags: go vet failed") ||
			!strings.Contains(err.Error(), "fmt.Printf format %d") {
			t.Errorf("checking %q got %v; want the release-only vet diagnostic", target, err)
		}
	}
	p.Build.ReleaseTags = nil
	if err := check(t.Context(), p, "", 0); err != nil {
		t.Fatalf("default variant: %v", err)
	}
}

func TestReleaseTagsSelectTSXEntries(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod": "module releasefixture\n\ngo 1.26\n",
		"local.go": "//go:build !release\n\npackage main\ninvalid syntax that a release build should never parse\n",
		"release.go": "//go:build release\n\npackage main\nimport \"" + compilerPath + "\"\nvar _ = compiler.App(src, \"Release.tsx\")\n",
		"Release.tsx": "import { mount, Text } from 'nsgo'; mount(() => <Text>Release</Text>);\n",
	})
	p := &project{Config: projectcfg.Config{Dir: dir}, buildReleaseTags: true, buildEmbed: true}
	p.Build.ReleaseTags = []string{"release"}
	tags, err := goTags(t.Context(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := p.bundleFile(t.Context(), nil, tags, nil)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(bundle)
	if err != nil || !strings.Contains(string(source), `compiler.Bundled("Release.tsx", `) {
		t.Fatalf("generated bundle = %s, %v", source, err)
	}
}
