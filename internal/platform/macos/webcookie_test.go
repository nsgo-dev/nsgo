//go:build darwin && !ios

package macos

import (
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strconv"
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

// TestWebCookiesListsAndClearWebsiteDataClears has a local server's page set an HttpOnly cookie, store a value in localStorage and make an IndexedDB database, with SetWebCookies writing cookies of example.test and other.test beside it. Once the web view is gone, WebCookies must list the page's cookie by its URL and the example.test ones by their domain, every field filled in. A second run, with no web view, must list them again, then ClearWebsiteData must leave the store empty and a page loaded after it must send no cookie and find neither the value nor the database. ClearWebsiteData empties the test binary's whole store, which WebKit keeps whatever HOME says, so the values are random and no other test's cookie is counted on after it.
func TestWebCookiesListsAndClearWebsiteDataClears(t *testing.T) {
	if phase := os.Getenv("NSGO_TEST_WEB_LIST_PROCESS"); phase != "" {
		cookieListApp(t, phase)
		return
	}
	var mu sync.Mutex
	heard := map[string]string{}
	value := rand.Text()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		heard[r.URL.Path] = r.Header.Get("Cookie")
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		// The page reports what it found and what it did once its storage settled.
		script := `const open = indexedDB.open('nsgo', 1); let made = false;
open.onupgradeneeded = () => { made = true; open.result.createObjectStore('s'); };
open.onsuccess = () => { open.result.close(); `
		if r.URL.Path == "/set" {
			w.Header().Set("Set-Cookie", "page="+value+"; HttpOnly; Path=/; Max-Age=120")
			script += `localStorage.setItem('k', '` + value + `'); window.nsgo.postMessage('stored'); };`
		} else {
			script += `window.nsgo.postMessage('storage ' + localStorage.getItem('k') + ' database ' + (made ? 'new' : 'kept')); };`
		}
		w.Write([]byte(`<!doctype html><script>` + script + `</script>`))
	}))
	defer server.Close()
	// Both runs set and look for the same expiry.
	expires := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	run := func(phase string) string {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEB_LIST_PROCESS="+phase, "NSGO_TEST_SERVER="+server.URL, "NSGO_TEST_VALUE="+value, "NSGO_TEST_EXPIRES="+expires)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", phase, err, out)
		}
		return string(out)
	}
	run("set")
	out := run("clear")
	mu.Lock()
	check := heard["/check"]
	mu.Unlock()
	if check != "" {
		t.Errorf("after ClearWebsiteData the page's request sent Cookie %q, want none", check)
	}
	if !strings.Contains(out, "storage null database new") {
		t.Errorf("after ClearWebsiteData the page did not find empty storage:\n%s", out)
	}
}

// cookieListApp is a run of TestWebCookiesListsAndClearWebsiteDataClears's app: "set" loads /set, sets the example.test cookies, unmounts the web view and lists; "clear" lists, clears, lists and loads /check.
func cookieListApp(t *testing.T, phase string) {
	server, value := os.Getenv("NSGO_TEST_SERVER"), os.Getenv("NSGO_TEST_VALUE")
	host := strings.TrimPrefix(server, "http://")
	host = host[:strings.LastIndexByte(host, ':')]
	at, _ := strconv.ParseInt(os.Getenv("NSGO_TEST_EXPIRES"), 10, 64)
	expires := time.Unix(at, 0).UTC()
	set := []core.WebCookie{
		{Name: "d", Value: value, Domain: ".example.test", Path: "/", Expires: expires, Secure: true, HTTPOnly: true, SameSite: core.WebSameSiteStrict},
		{Name: "w", Value: value, Domain: "www.example.test", Path: "/watch", Expires: expires},
		{Name: "o", Value: value, Domain: "other.test", Path: "/", Expires: expires},
	}
	// checkListed holds the store's listings to what the page and SetWebCookies wrote, and calls next once both answered.
	checkListed := func(cx *core.Scope, next func()) {
		cx.WebCookies(core.WebCookieFilter{URL: server + "/any"}, func(cookies []core.WebCookie, err error) {
			// Cookies go to a host whatever its port, so another test's for 127.0.0.1 may be listed too.
			i := slices.IndexFunc(cookies, func(c core.WebCookie) bool { return c.Name == "page" })
			if err != nil || i < 0 {
				t.Errorf("the page's URL listed %+v, %v; want its cookie", cookies, err)
				cx.Quit()
				return
			}
			c := cookies[i]
			if c.Value != value || c.Domain != host || c.Path != "/" || !c.HTTPOnly || c.Secure || time.Until(c.Expires) < time.Minute || time.Until(c.Expires) > 3*time.Minute {
				t.Errorf("the page's cookie listed as %+v", c)
			}
			cx.WebCookies(core.WebCookieFilter{Domain: "example.test"}, func(cookies []core.WebCookie, err error) {
				if err != nil || len(cookies) != 2 || cookies[0] != set[0] || cookies[1] != set[1] {
					t.Errorf("example.test listed %+v, %v; want %+v", cookies, err, set[:2])
				}
				next()
			})
		})
	}
	app := func(cx *core.Scope) core.Node {
		page := core.NewSignal(cx, "")
		switch phase {
		case "set":
			cx.SetWebCookies(set, func(err error) {
				if err != nil {
					t.Errorf("SetWebCookies: %v", err)
					cx.Quit()
					return
				}
				page.Set("/set")
			})
		case "clear":
			checkListed(cx, func() {
				cx.ClearWebsiteData(func(err error) {
					if err != nil {
						t.Errorf("ClearWebsiteData: %v", err)
					}
					cx.WebCookies(core.WebCookieFilter{}, func(cookies []core.WebCookie, err error) {
						if err != nil || len(cookies) != 0 {
							t.Errorf("after ClearWebsiteData the store listed %+v, %v", cookies, err)
						}
						page.Set("/check")
					})
				})
			})
		}
		message := func(m string) {
			t.Logf("%s", m)
			if m != "stored" {
				cx.Quit()
				return
			}
			// The page that set the cookie goes before the store is listed.
			page.Set("")
			checkListed(cx, cx.Quit)
		}
		return core.View{Children: []core.Node{
			core.Show{When: func() bool { return page.Get() != "" }, Then: func(*core.Scope) core.Node {
				return core.WebView{Source: core.WebSrc(server + page.Get()), Style: core.Style{Width: core.Pt(200), Height: core.Pt(150)}, OnMessage: message}
			}},
		}}
	}
	watchdog := time.AfterFunc(30*time.Second, func() { panic("the web view's store did not answer") })
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Web cookies", Width: 200, Height: 150}) })
}
