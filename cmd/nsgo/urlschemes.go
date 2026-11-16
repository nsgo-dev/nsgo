package main

import (
	"fmt"
	"regexp"
	"strings"
)

// schemePattern is RFC 3986's scheme, which Launch Services matches without regard to case.
var schemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)

// systemSchemes are schemes the system or a browser already handles; an app that claimed one would take its links from them, or never see them.
var systemSchemes = map[string]bool{
	"http": true, "https": true, "file": true, "ftp": true, "mailto": true, "tel": true, "sms": true, "facetime": true,
	"data": true, "about": true, "blob": true, "javascript": true, "asset": true, "x-apple-helpviewer": true,
}

// validateURLSchemes checks url_schemes, the schemes of the app's own links, which its macOS package registers with Launch Services so their links reach desktop.OnOpenLinks; it lowercases them.
func (p *project) validateURLSchemes() error {
	seen := map[string]bool{}
	for i, s := range p.URLSchemes {
		key := fmt.Sprintf("url_schemes[%d]", i)
		if !schemePattern.MatchString(s) {
			return fmt.Errorf("%s: %q is not a URL scheme; write myapp, not myapp://", key, s)
		}
		s = strings.ToLower(s)
		if systemSchemes[s] {
			return fmt.Errorf("%s: %q belongs to the system; choose a scheme of the app's own, as its name", key, s)
		}
		if seen[s] {
			return fmt.Errorf("%s: the scheme %q is listed twice", key, s)
		}
		seen[s] = true
		p.URLSchemes[i] = s
	}
	return nil
}

// urlSchemesPlist is the Info.plist entry that registers the app's URL schemes.
func (p *project) urlSchemesPlist() string {
	if len(p.URLSchemes) == 0 {
		return ""
	}
	return fmt.Sprintf("\t<key>CFBundleURLTypes</key>\n\t<array>\n\t\t<dict>\n\t\t\t<key>CFBundleURLName</key><string>%s</string>\n\t\t\t<key>CFBundleURLSchemes</key>%s\n\t\t</dict>\n\t</array>",
		xmlText(p.ID), plistStrings(p.URLSchemes))
}
