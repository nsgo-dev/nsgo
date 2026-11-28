package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/i18n"
	mobilelocales "github.com/nsgo-dev/nsgo/mobile/locales"
)

// shareCatalog is the package whose messages the share extension says, which an app's catalogs supply under its import path as they supply any of NSGo's.
const shareCatalog = nsgoModule + "/mobile/locales"

// shareStrings are the share extension's Localizable.strings keys, by the mobile/locales message each says; appName is the app's name.
var shareStrings = []string{"appName", "shareSaved", "shareFailed", "shareTooLarge", "shareDone", "shareCancel"}

// writeShareStrings writes a Localizable.strings into the share extension's .lproj directory for each language the app shows, holding its name and the extension's words with the app named, which the extension reads by the user's languages as UIKit picks a bundle's localization. The words are mobile/locales', unless the app's catalogs supply them. It returns the source language, which the extension falls back to.
func writeShareStrings(p *project, appex string) (string, error) {
	source, langs := mobilelocales.Source, []string{mobilelocales.Source}
	supplied := map[string]map[string]*catalogMessage{}
	if p.I18n.Dir != "" && hasCatalogs(p.I18n.Dir) {
		c, ds := loadCatalog(p.I18n.Dir, p.I18n.Source)
		if err := ds.err(); err != nil {
			return "", err
		}
		source, langs = c.source, c.languages()
		for lang, f := range c.files {
			for _, s := range f.supplies {
				if s.importPath == shareCatalog {
					supplied[lang] = s.msgs
				}
			}
		}
	}
	names, err := p.appNames()
	if err != nil {
		return "", err
	}
	for _, lang := range langs {
		name := p.Name
		if names != nil {
			if n, ok := names.names[lang]; ok {
				name = n
			} else {
				name = names.names[names.source]
			}
		}
		texts, err := shareTexts(lang, name, supplied)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, key := range shareStrings {
			fmt.Fprintf(&b, "%s = %s;\n", appleString(key), appleString(texts[key]))
		}
		dir := filepath.Join(appex, lang+".lproj")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, "Localizable.strings"), []byte(b.String()), 0o644); err != nil {
			return "", err
		}
	}
	return source, nil
}

// shareTexts are the share extension's words in lang, looked up as the app's own code looks mobile/locales' up: in what the app supplies, then the catalog's own, along the language's CLDR parents, else in the source.
func shareTexts(lang, name string, supplied map[string]map[string]*catalogMessage) (map[string]string, error) {
	own := mobilelocales.Catalog.Languages()
	langs := slices.Clone(own)
	for l := range supplied {
		if !slices.Contains(langs, l) {
			langs = append(langs, l)
		}
	}
	slices.Sort(langs[len(own):])
	matched := i18n.NewCatalog(langs[0], langs[1:]...).Match(lang)
	texts := map[string]string{"appName": name}
	for _, key := range shareStrings[1:] {
		found := false
		for l := matched; l != "" && !found; l = i18n.Parent(l) {
			if m := supplied[l][key]; m != nil && (m.text != "" || len(m.parts) > 0) {
				text, err := shareText(m, name)
				if err != nil {
					return nil, err
				}
				texts[key], found = text, true
			} else if slices.Contains(own, l) {
				texts[key], found = shareMessage(mobilelocales.In(l), key, name), true
			}
		}
		if !found {
			texts[key] = shareMessage(mobilelocales.In(mobilelocales.Source), key, name)
		}
	}
	return texts, nil
}

// shareMessage is one of mobile/locales' messages the share extension says.
func shareMessage(msgs mobilelocales.Messages, key, name string) string {
	switch key {
	case "shareSaved":
		return msgs.ShareSaved(name)
	case "shareFailed":
		return msgs.ShareFailed(name)
	case "shareTooLarge":
		return msgs.ShareTooLarge()
	case "shareDone":
		return msgs.ShareDone()
	case "shareCancel":
		return msgs.ShareCancel()
	}
	panic("nsgo: the share extension says no message " + key)
}

// shareText is a message the app supplies with {app} as its only argument, the app's name; the catalogs' checks have held it to the source's arguments.
func shareText(m *catalogMessage, name string) (string, error) {
	var b strings.Builder
	for _, part := range m.parts {
		switch part := part.(type) {
		case msgText:
			b.WriteString(string(part))
		case msgArg:
			if part.name != "app" || part.kind != "" {
				return "", fmt.Errorf("%s: the share extension writes only {app} in %s", shareCatalog, m.key)
			}
			b.WriteString(name)
		default:
			return "", fmt.Errorf("%s: the share extension writes %s without plurals or selects", shareCatalog, m.key)
		}
	}
	return b.String(), nil
}
