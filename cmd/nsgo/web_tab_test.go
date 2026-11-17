package main

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestWebTabNavigation runs testdata/keytab in a browser, where the test host's keys are trusted CDP input and Tab moves the focus by the browser's own navigation, which synthetic DOM key events never trigger. It covers the reproduction from #554.
func TestWebTabNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs native focus tests in a browser")
	}
	if _, err := findBrowser(); err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	p, err := loadForTest(filepath.Join("..", "..", "testdata", "keytab"), nil)
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
	var got []string
	for _, suite := range suites.Suites {
		for _, c := range suite.Cases {
			if c.Failure != nil || c.Skipped != nil {
				t.Fatalf("native focus test %s did not pass: %s", c.Name, data)
			}
			got = append(got, c.Name)
		}
	}
	slices.Sort(got)
	if want := []string{"TestTabMovesBetweenFields", "TestTabWalksNativeFocusStops"}; !slices.Equal(got, want) {
		t.Fatalf("reported tests = %v, want %v", got, want)
	}
}
