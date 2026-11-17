package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func checkFixture(t *testing.T) *project {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":  "module checkfixture\n\ngo 1.26\n",
		"main.go": "package fixture\nvar Value = 1\n",
	})
	p, err := loadOrDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// missingXcode also exercises a Mac whose selected Xcode has no mobile SDKs. The go command remains real, so the tests verify cross-target vetting.
func missingXcode(t *testing.T) {
	checkXcrun(t, "echo 'Xcode SDK unavailable' >&2\nexit 1\n")
}

func checkXcrun(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the unavailable Xcode fixture uses a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "xcrun"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCheckAllWithMissingWatchSDK(t *testing.T) {
	if testing.Short() || runtime.GOOS != "darwin" {
		t.Skip("cross-vets with real iOS and tvOS SDKs")
	}
	for _, name := range []string{"ios", "ios_simulator", "tvos", "tvos_simulator"} {
		if _, err := vetEnv(t.Context(), name); err != nil {
			t.Skipf("needs the %s SDK: %v", name, err)
		}
	}
	checkXcrun(t, "case \"$2\" in\n  watchos|watchsimulator) exit 1;;\nesac\nexec /usr/bin/xcrun \"$@\"\n")
	p := checkFixture(t)
	var err error
	out := captureStderr(t, func() { err = check(t.Context(), p, "all") })
	if err != nil {
		t.Fatalf("check with only watchOS missing: %v\n%s", err, out)
	}
	for _, name := range []string{"ios", "ios_simulator", "tvos", "tvos_simulator", "android_arm64", "web"} {
		if !strings.Contains(out, "vetted for "+name) {
			t.Errorf("did not vet available %s:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "skipped watchos, watchos_simulator: the watchOS SDK needs Xcode") || strings.Count(out, "skipped ") != 1 {
		t.Errorf("missing the unavailable SDK summary:\n%s", out)
	}
}

func TestCheckAllWithoutXcode(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-vets a real package")
	}
	missingXcode(t)
	p := checkFixture(t)
	var err error
	out := captureStderr(t, func() { err = check(t.Context(), p, "all") })
	if err != nil {
		t.Fatalf("check all without Xcode: %v\n%s", err, out)
	}
	for _, name := range []string{"macos_arm64", "macos_x64", "windows_arm64", "windows_x64", "linux_arm64", "linux_x64", "android_arm64", "web"} {
		if !strings.Contains(out, "vetted for "+name) {
			t.Errorf("did not vet %s:\n%s", name, out)
		}
	}
	reason := "they need macOS with Xcode"
	if runtime.GOOS == "darwin" {
		reason = "the iOS SDK needs Xcode: xcrun --sdk iphoneos --show-sdk-path"
	}
	if !strings.Contains(out, "skipped ios, ios_simulator, tvos, tvos_simulator, watchos, watchos_simulator: "+reason) || strings.Count(out, "skipped ") != 1 {
		t.Errorf("missing one summary of unavailable Apple targets and why:\n%s", out)
	}
}

func TestCheckExplicitAppleWithoutXcode(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-vets a real package")
	}
	missingXcode(t)
	p := checkFixture(t)
	for _, target := range []string{"ios", "ios_simulator", "tvos", "tvos_simulator", "watchos", "watchos_simulator", "ios,tvos", "all,ios", "ios,all"} {
		t.Run(target, func(t *testing.T) {
			var err error
			out := captureStderr(t, func() { err = check(t.Context(), p, target) })
			if err == nil || !strings.Contains(err.Error(), "Xcode") {
				t.Fatalf("explicit Apple target: %v\n%s", err, out)
			}
			if strings.Contains(target, "all") && !strings.Contains(out, "vetted for web") {
				t.Errorf("an explicit unavailable target prevented later checks:\n%s", out)
			}
			if target == "ios,tvos" && (!strings.Contains(err.Error(), "iOS") || !strings.Contains(err.Error(), "tvOS")) {
				t.Errorf("lost an explicit target's error: %v", err)
			}
		})
	}
}

func TestCheckTargetsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// Cancellation must fail before any SDK lookup, project access or skip.
	if err := checkTargets(ctx, nil, "all"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled check: %v", err)
	}
}

func TestCheckTargetFailuresContinue(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-vets a real package")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's Windows error must not affect the host vet")
	}
	p := checkFixture(t)
	writeFiles(t, p.Dir, map[string]string{
		"bad_windows.go": "package fixture\nvar _ = missingWindowsValue\n",
	})
	var err error
	out := captureStderr(t, func() { err = check(t.Context(), p, "windows_arm64,web") })
	if err == nil || !strings.Contains(err.Error(), "missingWindowsValue") || !strings.Contains(err.Error(), "windows_arm64") {
		t.Fatalf("target vet error was lost: %v\n%s", err, out)
	}
	if !strings.Contains(out, "vetted for web") {
		t.Errorf("a failed vet prevented the following target:\n%s", out)
	}
}
