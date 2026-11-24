package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitTemplates writes each template's app and runs its tests, Go's
// and TSX's, against this checkout of NSGo.
func TestInitTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("builds three apps")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range templates {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "app")
			if err := initApp(t.Context(), dir, "My App", "example.com/myapp", kind); err != nil {
				t.Fatal(err)
			}
			if err := initApp(t.Context(), dir, "My App", "", kind); err == nil {
				t.Error("init wrote over the app")
			}
			run := func(args ...string) {
				t.Helper()
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				// Out of any workspace, and with the module proxy on: a fresh runner's cache lacks go.mod files that tidy reads for modules the build never compiles, such as the old golang.org/x/sys esbuild requires.
				cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %v: %v\n%s", args, err, out)
				}
			}
			run("mod", "edit", "-require=github.com/nsgo-dev/nsgo@v0.0.0", "-replace=github.com/nsgo-dev/nsgo="+root)
			run("mod", "tidy")
			p, err := load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if kind != "go" {
				tsxTest, err := p.tsxTestFile()
				if err != nil || tsxTest == "" {
					t.Fatalf("no TSX test: %v", err)
				}
				ov, err := p.overlay(map[string]string{"zz_nsgo_tsx_test.go": tsxTest})
				if err != nil {
					t.Fatal(err)
				}
				run("test", "-overlay", ov, "-run", "TestTSX", ".")
				return
			}
			run("test", ".")
		})
	}
}

func TestInitBuiltinRejectsStandardLibraryModule(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=vendor")
	for _, module := range []string{"time", "archive", "syscall/js"} {
		t.Run(module, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "app")
			err := initApp(t.Context(), dir, "My App", module, "go")
			if err == nil || !strings.Contains(err.Error(), `-module "`+module+`" conflicts with Go standard library package`) ||
				!strings.Contains(err.Error(), "choose a different -module") {
				t.Errorf("init with -module %s: %v", module, err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("init created the destination: %v", err)
			}
		})
	}
}

func TestInitBuiltinAllowsNonconflictingModule(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app")
	const module = "time/clock"
	if err := initApp(t.Context(), dir, "My App", module, "go"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil || !strings.HasPrefix(string(got), "module "+module+"\n") {
		t.Errorf("go.mod = %q, %v; want module %s", got, err, module)
	}
}

func TestDirAppName(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("my.app", 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir("my.app")
	for dir, want := range map[string]string{".": "my-app", "": "my-app", "../2048 game": "game", "Hello World": "Hello World"} {
		if got, err := dirAppName(dir); err != nil || got != want {
			t.Errorf("dirAppName(%q) = %q, %v; want %q", dir, got, err, want)
		}
	}
	if _, err := dirAppName("../应用"); err == nil {
		t.Error("named an app after a directory with no letters it can use")
	}
}
