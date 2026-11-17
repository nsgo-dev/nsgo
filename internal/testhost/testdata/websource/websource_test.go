package websource_test

import (
	"slices"
	"sync"
	"testing"
	"time"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

type webEvent struct {
	kind string
	nav  core.WebNavigation
}

type webPage struct {
	mu     sync.Mutex
	events []webEvent
	errors []string
	scope  *core.Scope
	source *core.Signal[core.WebSource]
	ref    core.Ref
}

func startPage(t *testing.T, source core.WebSource) (*nsgotest.App, *webPage) {
	t.Helper()
	switch nsgotest.Target() {
	case "macos", "ios_simulator", "ios", "linux", "windows", "android":
	default:
		t.Skip("checks a native web view's load events and page contents")
	}
	p := new(webPage)
	note := func(kind string) func(core.WebNavigation) {
		return func(nav core.WebNavigation) {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.events = append(p.events, webEvent{kind, nav})
		}
	}
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		p.scope = cx
		p.source = core.NewSignal(cx, source)
		return core.WebView{
			SourceFunc: p.source.Get, Ref: &p.ref, TestID: "page",
			Style:       core.Style{Width: core.Pt(240), Height: core.Pt(140)},
			Headers:     map[string]string{"X-Test": "blank-source"},
			OnLoadStart: note("start"), OnLoad: note("load"), OnNavigationStateChange: note("navigation"),
			OnError: func(err error) {
				p.mu.Lock()
				defer p.mu.Unlock()
				p.errors = append(p.errors, err.Error())
			},
		}
	}, nsgotest.Size(260, 160))
	return a, p
}

func (p *webPage) loaded(t *testing.T, a *nsgotest.App, body string, blank bool) {
	t.Helper()
	a.WaitUntil(15*time.Second, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.errors) != 0 || slices.ContainsFunc(p.events, func(e webEvent) bool { return e.kind == "load" })
	})
	p.mu.Lock()
	events, errors := slices.Clone(p.events), slices.Clone(p.errors)
	p.events = nil
	p.mu.Unlock()
	if len(errors) != 0 {
		t.Fatalf("page errors: %v", errors)
	}
	if blank {
		var starts, loads int
		var loading, finished bool
		for _, event := range events {
			if event.nav.URL != "about:blank" {
				t.Errorf("blank page's %s reported %q", event.kind, event.nav.URL)
			}
			switch event.kind {
			case "start":
				starts++
				if !event.nav.Loading || loads != 0 {
					t.Errorf("load start came out of order or not loading: %+v", events)
				}
			case "load":
				loads++
				if event.nav.Loading || event.nav.Title != "" {
					t.Errorf("blank load reported %+v", event.nav)
				}
			case "navigation":
				loading = loading || event.nav.Loading
				finished = finished || !event.nav.Loading
			}
		}
		if starts != 1 || loads != 1 || !loading || !finished {
			t.Errorf("blank navigation reported %+v, want one start and load and both navigation states", events)
		}
	}
	var result string
	var evalErr error
	var answered bool
	p.scope.Post(func() {
		p.ref.Evaluate("document.body.textContent.trim()", func(text string, err error) {
			p.mu.Lock()
			defer p.mu.Unlock()
			result, evalErr, answered = text, err, true
		})
	})
	a.WaitUntil(10*time.Second, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return answered
	})
	p.mu.Lock()
	defer p.mu.Unlock()
	if evalErr != nil || result != body {
		t.Fatalf("page body = %s, %v; want %s", result, evalErr, body)
	}
}

func TestEmptyWebSourceStartsBlank(t *testing.T) {
	a, page := startPage(t, core.WebSource{})
	page.loaded(t, a, `""`, true)
}

func TestEmptyWebSourceClearsAPage(t *testing.T) {
	private := core.WebHTML("<title>Private</title><p>private</p>", "")
	a, page := startPage(t, private)
	page.loaded(t, a, `"private"`, false)
	page.scope.Post(func() { page.source.Set(core.WebSource{}) })
	page.loaded(t, a, `""`, true)
	page.scope.Post(func() { page.source.Set(private) })
	page.loaded(t, a, `"private"`, false)
	// Explicit loads also clear the page, and a repeated explicit load navigates again.
	for range 2 {
		page.scope.Post(func() { page.ref.Load(core.WebSource{}) })
		page.loaded(t, a, `""`, true)
	}
}
