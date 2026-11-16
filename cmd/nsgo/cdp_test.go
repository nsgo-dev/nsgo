package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBrowserStartupReportsExitAndCleansItsProfile(t *testing.T) {
	fakeBrowser(t, "echo 'browser startup rejected' >&2\nexit 23\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := startBrowser(ctx)
	if err == nil || !strings.Contains(err.Error(), "exit status 23") || !strings.Contains(err.Error(), "browser startup rejected") {
		t.Fatalf("startup error = %v, want the exit status and browser stderr", err)
	}
	checkBrowserProfileRemoved(t)
}

func TestBrowserStartupHonorsCancellation(t *testing.T) {
	fakeBrowser(t, "exec sleep 60\n")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	var err error
	go func() {
		_, err = startBrowser(ctx)
		close(done)
	}()
	t.Cleanup(func() { cancel(); <-done })
	// Cancel after the child has started and recorded its profile. Under
	// load, a fixed deadline can expire before its shell ever runs.
	ready := time.NewTicker(10 * time.Millisecond)
	defer ready.Stop()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	waiting := true
	for waiting {
		select {
		case <-done:
			t.Fatalf("browser exited before cancellation: %v", err)
		case <-timeout.C:
			t.Fatal("fake browser did not record its profile")
		case <-ready.C:
			profile, readErr := os.ReadFile(os.Getenv("NSGO_TEST_BROWSER_PROFILE"))
			waiting = readErr != nil || len(profile) == 0
		}
	}
	start := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("canceled browser did not stop within 3s")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("startup error = %v, want context cancellation", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("canceled browser took %s to stop", elapsed)
	}
	checkBrowserProfileRemoved(t)
}

func fakeBrowser(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake browser uses a POSIX shell")
	}
	dir := t.TempDir()
	t.Setenv("NSGO_TEST_BROWSER_PROFILE", filepath.Join(dir, "profile"))
	script := "#!/bin/sh\nfor arg do\ncase $arg in\n--user-data-dir=*) printf '%s' \"${arg#*=}\" > \"$NSGO_TEST_BROWSER_PROFILE\";;\nesac\ndone\n" + body
	path := filepath.Join(dir, "browser")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NSGO_BROWSER", path)
}

func checkBrowserProfileRemoved(t *testing.T) {
	t.Helper()
	profile, err := os.ReadFile(os.Getenv("NSGO_TEST_BROWSER_PROFILE"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(profile)); !os.IsNotExist(err) {
		t.Fatalf("browser profile still exists: %v", err)
	}
}
