package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// start runs the demo id as the website mounts it, in the machine's locale.
func start(t *testing.T, id string) *nsgotest.App { return startIn(t, "", id) }

// startIn runs the demo id as the website mounts it, in locale, or in the machine's where locale is "".
func startIn(t *testing.T, locale, id string) *nsgotest.App {
	h := core.NewHost()
	h.SetParams(map[string]any{"demo": id})
	return nsgotest.Start(t, in(locale), nsgotest.Host(h), nsgotest.Size(480, 320))
}

// locales are what the tests of the demos' words run in: the demos say them in the app's locale, which on a target is the machine's, so each test also forces English and Chinese to pass whatever the machine runs in.
var locales = []string{"", "en-US", "zh-CN"}

// in returns Root running in locale, or in the machine's where locale is "".
func in(locale string) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		if locale != "" {
			cx.SetLocale(locale)
		}
		return Root(cx)
	}
}

// eachLocale runs test once per locale.
func eachLocale(t *testing.T, test func(t *testing.T, locale string)) {
	for _, locale := range locales {
		name := locale
		if name == "" {
			name = "machine"
		}
		t.Run(name, func(t *testing.T) { test(t, locale) })
	}
}

func TestEveryDemoShows(t *testing.T) {
	for id := range demos {
		t.Run(id, func(t *testing.T) {
			app := start(t, id)
			if app.Exists("text:No preview " + id) {
				t.Fatal("the demo did not show")
			}
		})
	}
}

func TestButtonSays(t *testing.T) {
	eachLocale(t, func(t *testing.T, locale string) {
		app := startIn(t, locale, "button/basic")
		app.Press("text:/^(Save|保存)$/")
		app.WaitFor("text:/^(Saved|已保存)$/", time.Second)
	})
}

// TestButtonBusy shows a save running, its button busy until it is saved.
func TestButtonBusy(t *testing.T) {
	eachLocale(t, func(t *testing.T, locale string) {
		app := startIn(t, locale, "button/busy")
		const save, saved = "role:button /^(Save|保存)$/ >> nth=0", "text:/^(Saved|已保存)$/"
		app.Press(save)
		if !app.Find(save).Props.Busy {
			t.Fatal("the saving button is not busy")
		}
		app.Advance(2500 * time.Millisecond)
		app.Find(saved)
		if app.Find(save).Props.Busy {
			t.Fatal("the button stays busy once saved")
		}
	})
}

func TestPickerWithoutHost(t *testing.T) {
	eachLocale(t, func(t *testing.T, locale string) {
		app := nsgotest.Start(t, in(locale), nsgotest.Size(480, 320))
		app.Find("text:button/basic")
		app.Find("text:/^(Save|保存)$/")
	})
}

// previewMark is a code block's preview mark in the docs.
var previewMark = regexp.MustCompile("(?m)^```go\\b.*\\bpreview=(\\S+)")

// TestDocsNameDemos checks that every preview mark in the docs names a
// demo and every demo has an English page showing it.
func TestDocsNameDemos(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "public")
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		english := strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/en/")
		for _, m := range previewMark.FindAllStringSubmatch(string(data), -1) {
			if _, ok := demos[m[1]]; !ok {
				t.Errorf("%s marks preview=%s, which no demo is", path, m[1])
			}
			if english {
				used[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var unused []string
	for id := range demos {
		if !used[id] {
			unused = append(unused, id)
		}
	}
	slices.Sort(unused)
	if len(unused) > 0 {
		t.Errorf("no English page shows %v", unused)
	}
}
