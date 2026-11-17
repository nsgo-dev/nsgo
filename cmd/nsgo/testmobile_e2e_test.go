package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSimulatorProcessResults(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("NSGO_SIMULATOR_E2E") != "1" {
		t.Skip("set NSGO_SIMULATOR_E2E=1 to run tests in an iPhone simulator")
	}
	if runtime.GOARCH != "arm64" {
		t.Fatal("NSGo's simulator binaries are arm64; run this check on an Apple silicon Mac")
	}
	for _, fixture := range []string{"simulator", "simulatorplain", "simulatornohost"} {
		t.Run(fixture, func(t *testing.T) {
			p, err := loadForTest(filepath.Join("..", "..", "testdata", fixture), nil)
			if err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(t.TempDir(), "junit.xml")
			output := captureStderr(t, func() {
				err = runTests(t.Context(), p, testOptions{target: "ios_simulator", device: os.Getenv("NSGO_SIMULATOR_DEVICE"),
					run: "^Test(Fast|Crash)$", timeout: 30 * time.Second, junit: report})
			})
			if err == nil {
				t.Fatal("the panic was reported as a success")
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
			if suites.Tests != 2 || suites.Failures != 1 || len(suites.Suites) != 1 {
				t.Fatalf("test results: %s\n%s", data, output)
			}
			for _, result := range suites.Suites[0].Cases {
				switch result.Name {
				case "TestFast":
					if result.Failure != nil {
						t.Fatalf("fast test failed: %s", data)
					}
				case "TestCrash":
					if result.Failure == nil || !strings.Contains(result.Failure.Body, "panic: nsgo simulator crash fixture") || strings.Contains(result.Failure.Body, "exited with 0") {
						t.Fatalf("crash report: %s", data)
					}
				default:
					t.Fatalf("unexpected test result: %s", data)
				}
			}
		})
	}
}
