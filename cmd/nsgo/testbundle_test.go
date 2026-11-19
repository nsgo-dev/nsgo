package main

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/harmony"
)

func TestTestBundleID(t *testing.T) {
	short := testBundleID("dev.example.app")
	if short != "dev.example.app.nsgotest" {
		t.Fatalf("short test bundle = %q", short)
	}
	long := testBundleID(strings.Repeat("a", 56))
	if long == strings.Repeat("a", 56)+".nsgotest" {
		t.Fatal("long test bundle kept the app identity")
	}
	if _, err := harmony.BundleName(long); err != nil {
		t.Fatalf("fallback bundle = %q: %v", long, err)
	}
}
