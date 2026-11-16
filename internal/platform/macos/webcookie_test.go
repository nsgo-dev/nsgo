//go:build darwin && !ios

package macos

import (
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// TestSetWebCookiesReachesThePagesLoadedAfter sets an HttpOnly session cookie, an HttpOnly one with an expiry and one of another path before any web view exists, then shows a web view and an Incognito one of a local server's pages: the server must hear the first two from the web view and none from the Incognito one, and the page's script must not see the HttpOnly ones. A second run of the app sends the cookie with an expiry, which WebKit keeps in the test binary's store whatever HOME says and so lasts two minutes, but not the session one; the values are random, so what an earlier run left is no answer. Run ends a process's app for good, so each run is a process of its own.
func TestSetWebCookiesReachesThePagesLoadedAfter(t *testing.T) {
	if phase := os.Getenv("NSGO_TEST_WEB_PROCESS"); phase != "" {
		cookieApp(t, phase)
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
	home, value := t.TempDir(), rand.Text()
	run := func(phase string) string {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEB_PROCESS="+phase, "NSGO_TEST_SERVER="+server.URL, "NSGO_TEST_VALUE="+value, "HOME="+home)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", phase, err, out)
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

// cookieApp is a run of TestSetWebCookiesReachesThePagesLoadedAfter's app: "set" sets the cookies and loads /page and /incognito, "again" loads /again.
func cookieApp(t *testing.T, phase string) {
	server, value := os.Getenv("NSGO_TEST_SERVER"), os.Getenv("NSGO_TEST_VALUE")
	host := strings.TrimPrefix(server, "http://")
	host = host[:strings.LastIndexByte(host, ':')]
	app := func(cx *core.Scope) core.Node {
		ready := core.NewSignal(cx, phase == "again")
		loaded := 0
		quit := func(core.WebNavigation) {
			if loaded++; loaded == 1 && phase == "again" || loaded == 2 {
				cx.Quit()
			}
		}
		if phase == "set" {
			cx.SetWebCookies([]core.WebCookie{
				{Name: "sid", Value: value, Domain: host, HTTPOnly: true, SameSite: core.WebSameSiteLax},
				{Name: "keep", Value: value, Domain: host, Path: "/", Expires: time.Now().Add(2 * time.Minute), HTTPOnly: true},
				{Name: "elsewhere", Value: value, Domain: host, Path: "/elsewhere"},
			}, func(err error) {
				if err != nil {
					t.Errorf("SetWebCookies: %v", err)
					cx.Quit()
					return
				}
				ready.Set(true)
			})
		}
		size := core.Style{Width: core.Pt(200), Height: core.Pt(150)}
		message := func(m string) { t.Logf("%s", m) }
		return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
			core.Show{When: ready.Get, Then: func(*core.Scope) core.Node {
				if phase == "again" {
					return core.WebView{Source: core.WebSrc(server + "/again"), Style: size, OnLoad: quit}
				}
				return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
					core.WebView{Source: core.WebSrc(server + "/page"), Style: size, OnLoad: quit, OnMessage: message},
					core.WebView{Source: core.WebSrc(server + "/incognito"), Style: size, Incognito: true, OnLoad: quit},
				}}
			}},
		}}
	}
	watchdog := time.AfterFunc(30*time.Second, func() { panic("the web views did not load their pages") })
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Web cookies", Width: 400, Height: 150}) })
}
