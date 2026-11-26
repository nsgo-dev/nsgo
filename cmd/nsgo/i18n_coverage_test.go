package main

import (
	"strings"
	"testing"
)

// coverageSet is a catalog set holding an app whose fr file supplies
// supp, and a library whose own fr file holds libFr, of a, b and c.
func coverageSet(supp, libFr []string) (*catalogSet, *catalog) {
	messages := func(keys []string) map[string]*catalogMessage {
		msgs := map[string]*catalogMessage{}
		for _, key := range keys {
			msgs[key] = &catalogMessage{key: key}
		}
		return msgs
	}
	lib := &catalog{
		dir:        "lib",
		source:     "en",
		importPath: "example.com/lib",
		keys:       []string{"a", "b", "c"},
		files: map[string]*catalogFile{
			"en": {lang: "en", msgs: messages([]string{"a", "b", "c"})},
		},
	}
	if libFr != nil {
		lib.files["fr"] = &catalogFile{lang: "fr", msgs: messages(libFr)}
	}
	app := &catalog{
		dir:        "app",
		source:     "en",
		importPath: "example.com/app",
		app:        true,
		files: map[string]*catalogFile{
			"en": {lang: "en"},
			"fr": {lang: "fr", supplies: []*supply{{
				importPath: "example.com/lib",
				target:     lib,
				msgs:       messages(supp),
			}}},
		},
	}
	return &catalogSet{order: []*catalog{app, lib}}, app
}

func TestCoverageCountsKeys(t *testing.T) {
	for _, tt := range []struct {
		name   string
		supply []string
		libFr  []string
		want   string
	}{
		{"a supplied key covers itself alone", []string{"a"}, nil, "fr for 2 of 3 keys (b, c)"},
		{"every key supplied stops the warning", []string{"a", "b", "c"}, nil, ""},
		{"the library's own file covers its keys", nil, []string{"a", "b", "c"}, ""},
		{"nothing supplied warns of every key", nil, nil, "fr for all 3 keys"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cs, app := coverageSet(tt.supply, tt.libFr)
			cs.checkCoverage(app)
			if tt.want == "" {
				if len(cs.ds) != 0 {
					t.Fatalf("warnings: %v", cs.ds)
				}
				return
			}
			if len(cs.ds) != 1 {
				t.Fatalf("warnings: %v", cs.ds)
			}
			if got := cs.ds[0].String(); !strings.Contains(got, tt.want) {
				t.Errorf("warning = %q, want it to hold %q", got, tt.want)
			}
		})
	}
}

func TestLackingKeysShowsThree(t *testing.T) {
	if got, want := lackingKeys("fr", []string{"a", "b", "c", "d", "e"}, 6), "fr for 5 of 6 keys (a, b, c and 2 more)"; got != want {
		t.Errorf("lackingKeys = %q, want %q", got, want)
	}
}

func TestI18nCoverageSetting(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"nsgo.yaml": "name: App\ni18n:\n  dir: locales\n  source: en\n  coverage: off\n"})
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.I18n.Coverage != "off" {
		t.Errorf("coverage = %q, want off", p.I18n.Coverage)
	}
	dir = t.TempDir()
	writeFiles(t, dir, map[string]string{"nsgo.yaml": "name: App\ni18n:\n  coverage: silent\n"})
	_, err = load(dir)
	if err == nil || !strings.Contains(err.Error(), `i18n.coverage is "silent"; use warn or off`) {
		t.Errorf("a value other than warn or off: %v", err)
	}
}
