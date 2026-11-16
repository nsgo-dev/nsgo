//go:build linux && !android

package linux

import (
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestSetWebCookiesReachesThePagesLoadedAfter sets an HttpOnly session cookie, an HttpOnly one with an expiry and one of another path before any web view exists, then shows a web view and an Incognito one of a local server's pages: the server must hear the first two from the web view and none from the Incognito one, and the page's script must not see the HttpOnly ones. A second run of the app, with the same website data, sends the cookie with an expiry but not the session one. WebKit keeps the first thread that uses it, so each run is a process of its own; it skips where WebKitGTK 6.0 is missing.
func TestSetWebCookiesReachesThePagesLoadedAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if phase := os.Getenv("NSGO_TEST_WEBKIT_PROCESS"); strings.HasPrefix(phase, t.Name()+":") {
		cookieApp(t, strings.TrimPrefix(phase, t.Name()+":"))
		return
	}
	var mu sync.Mutex
	heard := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		heard[r.URL.Path] = r.Header.Get("Cookie")
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><script>window.nsgo.postMessage('script sees ' + document.cookie);</script>`))
	}))
	defer server.Close()
	dir, value := t.TempDir(), rand.Text()
	run := func(phase string) string {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEBKIT_PROCESS="+t.Name()+":"+phase, "NSGO_TEST_SERVER="+server.URL, "NSGO_TEST_VALUE="+value,
			"XDG_DATA_HOME="+filepath.Join(dir, "data"), "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", phase, err, out)
		}
		if strings.Contains(string(out), "--- SKIP") {
			t.Skipf("%s", out)
		}
		return string(out)
	}
	out := run("set")
	mu.Lock()
	page, incognito := heard["/page"], heard["/incognito"]
	mu.Unlock()
	for _, want := range []string{"sid=" + value, "keep=" + value} {
		if !strings.Contains(page, want) {
			t.Errorf("the web view's request sent Cookie %q, want %s in it", page, want)
		}
	}
	if strings.Contains(page, "elsewhere=") {
		t.Errorf("the web view's request sent Cookie %q, with the cookie of another path", page)
	}
	if incognito != "" {
		t.Errorf("the Incognito web view's request sent Cookie %q, want none", incognito)
	}
	// The value is in no other line the app logs.
	if !strings.Contains(out, "script sees") || strings.Contains(out, value) {
		t.Errorf("the page's script did not report, or saw the HttpOnly cookies:\n%s", out)
	}
	run("again")
	mu.Lock()
	again := heard["/again"]
	mu.Unlock()
	if !strings.Contains(again, "keep="+value) {
		t.Errorf("after a restart the web view sent Cookie %q, want keep=%s in it", again, value)
	}
	if strings.Contains(again, "sid=") {
		t.Errorf("after a restart the web view sent Cookie %q, with the session cookie", again)
	}
}

// cookieApp is a run of TestSetWebCookiesReachesThePagesLoadedAfter's app: "set" sets the cookies and loads /page and /incognito, "again" loads /again; the window's title says when it is done.
func cookieApp(t *testing.T, phase string) {
	if err := openWebKit(); err != nil {
		t.Skip(err)
	}
	server, value := os.Getenv("NSGO_TEST_SERVER"), os.Getenv("NSGO_TEST_VALUE")
	host := strings.TrimPrefix(server, "http://")
	host = host[:strings.LastIndexByte(host, ':')]
	title := "nsgo-native-cookies-" + phase
	app := func(cx *core.Scope) core.Node {
		ready := core.NewSignal(cx, phase == "again")
		loaded := core.NewSignal(cx, 0)
		failure, script := core.NewSignal(cx, ""), core.NewSignal(cx, "")
		cx.Effect(func() {
			if failure.Get() != "" {
				cx.SetTitle(title + "|" + failure.Get())
				return
			}
			want := 2
			if phase == "again" {
				want = 1
			}
			if loaded.Get() >= want && (phase == "again" || script.Get() != "") {
				cx.SetTitle(title + "|done")
			}
		})
		load := func(core.WebNavigation) { loaded.Update(func(n int) int { return n + 1 }) }
		if phase == "set" {
			cx.SetWebCookies([]core.WebCookie{
				{Name: "sid", Value: value, Domain: host, HTTPOnly: true, SameSite: core.WebSameSiteLax},
				{Name: "keep", Value: value, Domain: host, Path: "/", Expires: time.Now().Add(time.Hour), HTTPOnly: true},
				{Name: "elsewhere", Value: value, Domain: host, Path: "/elsewhere"},
			}, func(err error) {
				if err != nil {
					failure.Set(err.Error())
					return
				}
				ready.Set(true)
			})
		}
		size := core.Style{Width: driver.Pt(150), Height: driver.Pt(100)}
		message := func(m string) {
			t.Logf("%s", m)
			script.Set(m)
		}
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(120), Direction: core.Row}, Children: []core.Node{
			core.Show{When: ready.Get, Then: func(*core.Scope) core.Node {
				if phase == "again" {
					return core.WebView{Source: core.WebSrc(server + "/again"), Style: size, OnLoad: load}
				}
				return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
					core.WebView{Source: core.WebSrc(server + "/page"), Style: size, OnLoad: load, OnMessage: message},
					core.WebView{Source: core.WebSrc(server + "/incognito"), Style: size, Incognito: true, OnLoad: load},
				}}
			}},
		}}
	}
	w := startTestWindow(t, app, Options{ID: "nsgo-test-web-cookies", Title: title, Width: 320, Height: 120})
	waitForTitle(t, w, title+"|done")
}
