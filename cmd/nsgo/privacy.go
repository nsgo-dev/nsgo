package main

import (
	"os"
	"path/filepath"
)

const privacyManifestName = "PrivacyInfo.xcprivacy"

var nsgoPrivacyReasons = map[string][]string{
	"NSPrivacyAccessedAPICategoryUserDefaults":   {"CA92.1"},
	"NSPrivacyAccessedAPICategoryFileTimestamp":  {"C617.1"},
	"NSPrivacyAccessedAPICategorySystemBootTime": {"35F9.1"},
}

func (p *project) privacyManifest() []byte {
	apis := map[string][]string{}
	for name, reasons := range nsgoPrivacyReasons {
		apis[name] = append([]string(nil), reasons...)
	}
	for name, reasons := range p.Apple.Privacy.AccessedAPIs {
		apis[name] = append(apis[name], reasons...)
	}
	accessed := make([]any, 0, len(apis))
	for name, reasons := range apis {
		values := make([]any, len(reasons))
		for i, reason := range reasons {
			values[i] = reason
		}
		accessed = append(accessed, map[string]any{
			"NSPrivacyAccessedAPIType":        name,
			"NSPrivacyAccessedAPITypeReasons": values,
		})
	}
	tracking := false
	if p.Apple.Privacy.Tracking != nil {
		tracking = *p.Apple.Privacy.Tracking
	}
	domains := make([]any, len(p.Apple.Privacy.TrackingDomains))
	for i, domain := range p.Apple.Privacy.TrackingDomains {
		domains[i] = domain
	}
	data := make([]any, len(p.Apple.Privacy.CollectedData))
	for i, item := range p.Apple.Privacy.CollectedData {
		data[i] = item
	}
	return encodePlist(map[string]any{
		"NSPrivacyAccessedAPITypes":   accessed,
		"NSPrivacyCollectedDataTypes": data,
		"NSPrivacyTracking":           tracking,
		"NSPrivacyTrackingDomains":    domains,
	})
}

func (p *project) writePrivacyManifest(bundle string) error {
	return os.WriteFile(filepath.Join(bundle, privacyManifestName), p.privacyManifest(), 0o644)
}
