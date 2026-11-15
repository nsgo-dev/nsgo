package harmony_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	bridgeCall = regexp.MustCompile(`bridge\.Call\("([A-Za-z]+)"`)
	methodList = regexp.MustCompile(`methodList: \[([^\]]*)\]`)
	bridgeDecl = regexp.MustCompile(`^\s{2}([a-z][A-Za-z]*)\((.*)\): (void|string) \{`)
)

// TestBridgeMethodsAreProxied checks that every method the Go renderer calls
// on nsgoHarmony is in javaScriptProxy's methodList and declared on the
// Bridge class. A method missing from the list is undefined in the page's
// global, so a Call panics the module with "property X is not a function" and
// the app dies: keepawake was added to Bridge and to Go but not to the list.
func TestBridgeMethodsAreProxied(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	page := read(t, filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "ets", "pages", "Index.ets"))

	listed := map[string]bool{}
	m := methodList.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("javaScriptProxy has no methodList: the pattern no longer matches")
	}
	for _, name := range strings.Split(m[1], ",") {
		name = strings.Trim(strings.TrimSpace(name), "'\"")
		if name != "" {
			listed[name] = true
		}
	}
	if len(listed) == 0 {
		t.Fatal("methodList is empty: the pattern no longer matches")
	}

	// The methods Bridge declares, from its class body.
	start := strings.Index(page, "class Bridge {")
	if start < 0 {
		t.Fatal("Index.ets has no Bridge class")
	}
	body := page[start:]
	if end := strings.Index(body, "\n}\n"); end > 0 {
		body = body[:end]
	}
	declared := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		if d := bridgeDecl.FindStringSubmatch(line); d != nil {
			declared[d[1]] = true
		}
	}

	// Every method Go calls.
	called := map[string]bool{}
	for _, f := range []string{"mount.go", "harmonyos.go", "canvas.go", "text.go", "folders.go"} {
		for _, c := range bridgeCall.FindAllStringSubmatch(read(t, filepath.Join(root, "internal", "platform", "harmonyos", f)), -1) {
			called[c[1]] = true
		}
	}
	if len(called) == 0 {
		t.Fatal("the renderer calls no bridge method: the pattern no longer matches")
	}

	for name := range called {
		if !declared[name] {
			t.Errorf("Bridge has no %s(), which the renderer calls", name)
		}
		if !listed[name] {
			t.Errorf("javaScriptProxy's methodList has no %s: calling it panics the module", name)
		}
	}
	for name := range listed {
		if !declared[name] {
			t.Errorf("methodList names %s, which Bridge does not declare", name)
		}
	}
}
