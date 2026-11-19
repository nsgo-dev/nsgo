package main

import (
	"strings"
	"testing"
)

func TestPrivacyManifestIncludesNSGoAndAppDeclarations(t *testing.T) {
	tracking := true
	p := &project{}
	p.Apple.Privacy.AccessedAPIs = map[string][]string{"NSPrivacyAccessedAPICategoryDiskSpace": {"E174.1"}}
	p.Apple.Privacy.Tracking = &tracking
	p.Apple.Privacy.TrackingDomains = []string{"example.com"}
	data := string(p.privacyManifest())
	for _, want := range []string{"NSPrivacyAccessedAPICategoryUserDefaults", "CA92.1", "NSPrivacyAccessedAPICategoryDiskSpace", "E174.1", "NSPrivacyTracking", "example.com"} {
		if !strings.Contains(data, want) {
			t.Errorf("privacy manifest lacks %q:\n%s", want, data)
		}
	}
}
