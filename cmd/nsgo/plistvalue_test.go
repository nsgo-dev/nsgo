package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"go.yaml.in/yaml/v3"
)

func TestAppleInfoArrays(t *testing.T) {
	var p project
	if err := yaml.Unmarshal([]byte("name: PiP\nid: dev.example.pip\napple:\n  info:\n    UIBackgroundModes: [audio]\n    NSMicrophoneUsageDescription: 'A & B'\n    ExampleBoolean: true\n"), &p); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"ios", "macos", "tvos", "watchos"} {
		t.Run(target, func(t *testing.T) {
			templates := map[string]*template.Template{"ios": iosPlist, "macos": macPlist, "tvos": tvosPlist, "watchos": watchosPlist}
			path := filepath.Join(t.TempDir(), "Info.plist")
			if err := writePlist(path, &p, templates[target], ""); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"<key>UIBackgroundModes</key><array><string>audio</string></array>", "<string>A &amp; B</string>", "<key>ExampleBoolean</key><true/>"} {
				if !strings.Contains(string(b), want) {
					t.Fatalf("missing %s in %s", want, b)
				}
			}
		})
	}
	p.Apple.Info["Bad"] = make(chan int)
	if err := writePlist(filepath.Join(t.TempDir(), "Info.plist"), &p, iosPlist, ""); err == nil {
		t.Fatal("unsupported plist value was stringified")
	}
}

func TestSensorAppleInfoDictionaryAndLocationEntitlement(t *testing.T) {
	var p project
	if err := yaml.Unmarshal([]byte("apple:\n  info:\n    NSLocationWhenInUseUsageDescription: Nearby places\n    UIBackgroundModes: [location]\n    NSLocationTemporaryUsageDescriptionDictionary:\n      route: A precise route\n"), &p); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := writePlist(path, &p, iosPlist, ""); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "<key>NSLocationTemporaryUsageDescriptionDictionary</key><dict><key>route</key><string>A precise route</string></dict>") {
		t.Fatalf("dictionary missing: %s", b)
	}
	if p.hardenedEntitlements()["com.apple.security.personal-information.location"] != true {
		t.Fatal("location usage omitted hardened runtime entitlement")
	}
	p.Apple.Info = nil
	if len(p.hardenedEntitlements()) != 0 {
		t.Fatal("empty build claims an entitlement")
	}
}
