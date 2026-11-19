package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAndroidProcessExitResults(t *testing.T) {
	if os.Getenv("NSGO_ANDROID_E2E") != "1" {
		t.Skip("set NSGO_ANDROID_E2E=1 to run tests on an Android device")
	}
	for _, early := range []bool{false, true} {
		name := "hosted"
		if early {
			name = "before_host"
		}
		t.Run(name, func(t *testing.T) {
			p, err := loadForTest(filepath.Join("..", "..", "testdata", "androidexit"), nil)
			if err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(t.TempDir(), "junit.xml")
			opts := testOptions{target: "android", device: os.Getenv("NSGO_ANDROID_DEVICE"),
				run: "^Test(Pass|Crash|Exit)$", timeout: 30 * time.Second, junit: report}
			if early {
				opts.run, opts.env = "^TestPass$", []string{"NSGO_EXIT_BEFORE_HOST=1"}
			}
			output := captureStderr(t, func() { err = runTests(t.Context(), p, opts) })
			if err == nil {
				t.Fatal("unmarked process exits were reported as success")
			}
			runErr := err
			data, err := os.ReadFile(report)
			if err != nil {
				t.Fatalf("report: %v; run: %v\n%s", err, runErr, output)
			}
			var suites junitSuites
			if err := xml.Unmarshal(data, &suites); err != nil {
				t.Fatal(err)
			}
			tests, failures := 3, 2
			if early {
				tests, failures = 1, 1
			}
			if suites.Tests != tests || suites.Failures != failures || len(suites.Suites) != 1 {
				t.Fatalf("test results: %s\n%s", data, output)
			}
			for _, result := range suites.Suites[0].Cases {
				elapsed, err := strconv.ParseFloat(result.Time, 64)
				if err != nil || elapsed >= 20 {
					t.Fatalf("process result waited for the timeout: %s", data)
				}
				if result.Name == "TestPass" && !early {
					if result.Failure != nil {
						t.Fatalf("normal run failed: %s", data)
					}
					continue
				}
				if result.Failure == nil {
					t.Fatalf("process exit passed: %s", data)
				}
				status := "1"
				if early {
					status = "0"
				} else if result.Name == "TestCrash" {
					status = "2"
					if !strings.Contains(result.Failure.Body, "panic: android exit fixture") {
						t.Fatalf("panic lost: %s", data)
					}
				}
				if !strings.Contains(result.Failure.Body+result.Failure.Message, "status "+status) || strings.Contains(result.Failure.Body, "took over") {
					t.Fatalf("wrong failure: %s", data)
				}
			}
			// The tests ran under their own package, removed once they ended, so an installed build of the app is never replaced (#812).
			d, err := adbDevice(t.Context(), opts.device)
			if err != nil {
				t.Fatal(err)
			}
			left, err := d.output(t.Context(), "shell", "pm", "list", "packages", testBundleID(p.ID))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(left) != "" {
				t.Fatalf("the test build stayed installed: %s", left)
			}
		})
	}
}
