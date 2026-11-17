package main

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWebCanvasFontArrival(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a canvas font test in a browser")
	}
	if _, err := findBrowser(); err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	p, err := loadForTest(filepath.Join("testdata", "canvasfont"), nil)
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "tests.xml")
	if err := runTests(ctx, p, testOptions{target: "web", junit: report, timeout: time.Minute}); err != nil {
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
	if suites.Tests != 1 || suites.Failures != 0 {
		t.Fatalf("browser results: %s", data)
	}
	for _, suite := range suites.Suites {
		for _, c := range suite.Cases {
			if c.Skipped != nil {
				t.Fatalf("canvas font test skipped: %+v", c.Skipped)
			}
		}
	}
}
