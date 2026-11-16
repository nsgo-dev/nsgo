package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestURLSchemesRegisterInThePlist(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"nsgo.yaml": "name: Umapp\nid: me.oneo.umapp\nurl_schemes: [Umapp, umapp-dev]\n"})
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(p.URLSchemes, ","); got != "umapp,umapp-dev" {
		t.Errorf("schemes %q", got)
	}
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := writePlist(path, p, macPlist, p.urlSchemesPlist()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	plist := string(data)
	for _, want := range []string{
		"<key>CFBundleURLTypes</key>",
		"<key>CFBundleURLName</key><string>me.oneo.umapp</string>",
		"<key>CFBundleURLSchemes</key><array><string>umapp</string><string>umapp-dev</string></array>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("Info.plist lacks %q:\n%s", want, plist)
		}
	}
	if lint, err := exec.LookPath("plutil"); err == nil {
		if out, err := exec.Command(lint, "-lint", path).CombinedOutput(); err != nil {
			t.Errorf("plutil: %s", out)
		}
	}
}

func TestNoURLSchemesWriteNothing(t *testing.T) {
	if got := (&project{ID: "me.oneo.umapp"}).urlSchemesPlist(); got != "" {
		t.Errorf("an app without schemes registers %q", got)
	}
}

func TestURLSchemesAreChecked(t *testing.T) {
	for schemes, want := range map[string]string{
		"[umapp://]":     "not a URL scheme",
		"[1app]":         "not a URL scheme",
		"[https]":        "belongs to the system",
		"[MailTo]":       "belongs to the system",
		"[umapp, UMAPP]": "listed twice",
	} {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"nsgo.yaml": "name: Umapp\nurl_schemes: " + schemes + "\n"})
		if _, err := load(dir); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", schemes, err, want)
		}
	}
}
