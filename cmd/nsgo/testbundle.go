package main

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/harmony"
)

// testProject gives a target test its own installed application identity, so
// installing a test build never replaces the app the developer is testing.
func testProject(p *project) *project {
	t := *p
	t.ID = testBundleID(p.ID)
	return &t
}

func testBundleID(id string) string {
	candidate := id + ".nsgotest"
	if _, err := harmony.BundleName(candidate); err == nil {
		return candidate
	}
	sum := sha256.Sum256([]byte(id))
	return "dev.nsgo.test." + hex.EncodeToString(sum[:8])
}
