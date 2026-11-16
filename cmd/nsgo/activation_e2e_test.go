package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMacOSActivationEndToEnd runs testdata/activation in the native host, not as a headless library test in a target runtime: without -activate its field tests fail naming the inactive app while its Tab test passes in the background, and with -activate all pass. Opt in on an unlocked Mac with NSGO_TARGET_E2E=1.
func TestMacOSActivationEndToEnd(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("NSGO_TARGET_E2E") == "" {
		t.Skip("set NSGO_TARGET_E2E=1 on an unlocked Mac: the test activates its app")
	}
	p, err := loadForTest(filepath.Join("..", "..", "testdata", "activation"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"background", "activate"} {
		t.Run(name, func(t *testing.T) {
			activate := name == "activate"
			junit := filepath.Join(t.TempDir(), "results.xml")
			err := runTests(t.Context(), p, testOptions{target: "macos", activate: activate, junit: junit, env: []string{"NSGO_TEST_ACTIVATE=0"}})
			data, readErr := os.ReadFile(junit)
			if readErr != nil {
				t.Fatal(readErr)
			}
			var report junitSuites
			if e := xml.Unmarshal(data, &report); e != nil {
				t.Fatal(e)
			}
			if activate {
				if err != nil || report.Tests != 3 || report.Failures != 0 {
					t.Fatalf("with -activate: %v\n%s", err, data)
				}
			} else {
				if err == nil || report.Tests != 3 || report.Failures != 2 {
					t.Fatalf("without -activate: %v\n%s", err, data)
				}
				// The two field tests click an inactive app's field and must name the cause; Tab and Space still reach a background window's responder.
				for _, s := range report.Suites {
					for _, c := range s.Cases {
						refused := c.Failure != nil && strings.Contains(c.Failure.Message+c.Failure.Body, "test app is not active") && strings.Contains(c.Failure.Message+c.Failure.Body, "-target macos -activate")
						if refused != (c.Name != "TestTabAndKey") {
							t.Errorf("without -activate, %s: failure %+v", c.Name, c.Failure)
						}
					}
				}
			}
		})
	}
}
