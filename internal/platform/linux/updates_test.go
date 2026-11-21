//go:build linux && !android && !nsgo_steam

package linux

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCommand puts a shell script named name into dir.
func fakeCommand(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestSelfUpdatableAsksThePackageManagerThatOwnsTheApp(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	// dpkg owns the running executable; pacman owns nothing.
	fakeCommand(t, bin, "dpkg-query", `[ "$1" = --search ] && [ "$2" = '`+dpkgPattern.Replace(exe)+`' ]`)
	fakeCommand(t, bin, "pacman", "exit 1")
	t.Setenv("PATH", bin)
	ctx := context.Background()
	for download, want := range map[string]bool{
		"https://example.com/v1.2/hello-1.2-linux-x64.deb":         true,
		"https://example.com/v1.2/hello-1.2-linux-x64.pkg.tar.gz":  false,
		"https://example.com/v1.2/hello-1.2-linux-x64.AppImage":    false,
		"https://example.com/v1.2/hello-1.2-linux-x64.tar.gz":      false,
		"https://example.com/v1.2/hello-1.2-linux-x64.deb?x=1.zip": true,
	} {
		if got := selfUpdatable(ctx, download); got != want {
			t.Errorf("selfUpdatable(%q) = %v, want %v", download, got, want)
		}
	}
	// A system without dpkg, as Arch and Fedora are, installs no Debian package.
	t.Setenv("PATH", t.TempDir())
	if selfUpdatable(ctx, "https://example.com/hello-1.2-linux-x64.deb") {
		t.Error("a Debian package is installable without dpkg")
	}
}

func TestInstallUpdateRunsThePackageManager(t *testing.T) {
	bin, dir := t.TempDir(), t.TempDir()
	log := filepath.Join(dir, "pkexec.log")
	fakeCommand(t, bin, "pkexec", `echo "$@" >> '`+log+`'`)
	t.Setenv("PATH", bin)
	install := func(path string) error {
		errs := make(chan error, 1)
		installUpdate(path, func(err error) { errs <- err })
		return <-errs
	}
	for _, path := range []string{"/cache/updates/hello-1.2-linux-x64.deb", "/cache/updates/hello-1.2-linux-x64.pkg.tar.gz"} {
		if err := install(path); err != nil {
			t.Errorf("install %s: %v", path, err)
		}
	}
	if err := install("/cache/updates/hello-1.2-linux-x64.tar.gz"); err == nil {
		t.Error("a plain archive installed")
	}
	got, _ := os.ReadFile(log)
	want := "dpkg --install /cache/updates/hello-1.2-linux-x64.deb\npacman --upgrade --noconfirm /cache/updates/hello-1.2-linux-x64.pkg.tar.gz\n"
	if string(got) != want {
		t.Errorf("pkexec ran %q, want %q", got, want)
	}
	// A refused authentication, which pkexec answers with 126, fails the install.
	fakeCommand(t, bin, "pkexec", "exit 126")
	if err := install("/cache/updates/hello-1.2-linux-x64.deb"); err == nil || !strings.Contains(err.Error(), "126") {
		t.Errorf("a refused authentication answered %v", err)
	}
}
