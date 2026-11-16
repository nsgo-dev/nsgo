package core_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// cookieAccess allows the cookies of hosts and nothing else.
type cookieAccess struct{ hosts []string }

func (a cookieAccess) Allows(req core.AccessRequest) bool {
	if req.Permission != core.AccessWebView || req.Method != core.AccessCookies {
		return false
	}
	for _, h := range a.hosts {
		if strings.HasPrefix(req.Resource, "https://"+h+"/") || strings.HasPrefix(req.Resource, "http://"+h+"/") {
			return true
		}
	}
	return false
}

func (cookieAccess) WebHosts() []string { return nil }

func startCookies(t *testing.T, access core.Access) (*nsgotest.App, *core.Scope) {
	t.Helper()
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		if access != nil {
			cx.ProvideAccess(access)
		}
		return core.View{}
	})
	return app, scope
}

func TestSetWebCookiesKeepsThemInTheStore(t *testing.T) {
	app, cx := startCookies(t, nil)
	expires := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	calls := 0
	var got error
	set := func(cookies ...core.WebCookie) {
		cx.SetWebCookies(cookies, func(err error) { got = err; calls++ })
		app.Settle()
	}
	set(core.WebCookie{Name: "SID", Value: "a", Domain: ".Example.com", HTTPOnly: true, Secure: true, SameSite: core.WebSameSiteNone, Expires: expires},
		core.WebCookie{Name: "pref", Value: "x", Domain: "www.example.com", Path: "/watch"})
	set(core.WebCookie{Name: "SID", Value: "b", Domain: ".example.com", Path: "/", HTTPOnly: true})
	if calls != 2 || got != nil {
		t.Fatalf("done ran %d times, last with %v", calls, got)
	}
	want := []core.WebCookie{
		{Name: "pref", Value: "x", Domain: "www.example.com", Path: "/watch"},
		{Name: "SID", Value: "b", Domain: ".example.com", Path: "/", HTTPOnly: true},
	}
	if !slices.Equal(app.WebCookies, want) {
		t.Fatalf("store %+v, want %+v", app.WebCookies, want)
	}
}

func TestSetWebCookiesRefusesWhatNoPlatformTakes(t *testing.T) {
	app, cx := startCookies(t, nil)
	for _, c := range []core.WebCookie{
		{Value: "v", Domain: "example.com"},
		{Name: "a b", Domain: "example.com"},
		{Name: "a", Value: "x;y", Domain: "example.com"},
		{Name: "a", Domain: ""},
		{Name: "a", Domain: "."},
		{Name: "a", Domain: "https://example.com"},
		{Name: "a", Domain: "example.com:443"},
		{Name: "a", Domain: ".127.0.0.1"},
		{Name: "a", Domain: "example.com", Path: "watch"},
		{Name: "a", Domain: "example.com", SameSite: core.WebSameSiteNone},
		{Name: "__Secure-a", Domain: "example.com"},
		{Name: "__Host-a", Domain: ".example.com", Secure: true},
		{Name: "__Host-a", Domain: "example.com", Path: "/x", Secure: true},
	} {
		var got error
		ran := false
		// A valid cookie before it goes nowhere either.
		cx.SetWebCookies([]core.WebCookie{{Name: "ok", Domain: "example.com"}, c}, func(err error) { got, ran = err, true })
		if !ran || got == nil {
			t.Errorf("%+v: done ran %v with %v, want an error at once", c, ran, got)
		}
	}
	app.Settle()
	if len(app.WebCookies) != 0 {
		t.Errorf("store %+v, want nothing", app.WebCookies)
	}
	// What the platforms take.
	for _, c := range []core.WebCookie{
		{Name: "a", Domain: "127.0.0.1"},
		{Name: "a", Domain: "::1"},
		{Name: "__Host-a", Domain: "example.com", Secure: true},
		{Name: "a", Value: "with spaces, commas and \"quotes\"", Domain: "localhost"},
	} {
		var got error
		cx.SetWebCookies([]core.WebCookie{c}, func(err error) { got = err })
		app.Settle()
		if got != nil {
			t.Errorf("%+v: %v", c, got)
		}
	}
}

func TestSetWebCookiesReportsThePlatform(t *testing.T) {
	app, cx := startCookies(t, nil)
	app.SetWebCookies = func([]core.WebCookie) error { return errors.New("the store is locked") }
	var got error
	cx.SetWebCookies([]core.WebCookie{{Name: "a", Domain: "example.com"}}, func(err error) { got = err })
	app.Settle()
	if got == nil || got.Error() != "the store is locked" || len(app.WebCookies) != 0 {
		t.Fatalf("done heard %v, store %+v", got, app.WebCookies)
	}
	ran := false
	cx.SetWebCookies(nil, func(err error) { ran = err == nil })
	if !ran {
		t.Error("no cookies did not answer at once")
	}
}

func TestSetWebCookiesHeldToAccess(t *testing.T) {
	app, cx := startCookies(t, cookieAccess{hosts: []string{"example.com"}})
	var got error
	cx.SetWebCookies([]core.WebCookie{{Name: "a", Domain: "example.com"}}, func(err error) { got = err })
	app.Settle()
	if got != nil || len(app.WebCookies) != 1 {
		t.Fatalf("allowed: %v, store %+v", got, app.WebCookies)
	}
	cx.SetWebCookies([]core.WebCookie{{Name: "b", Domain: "example.com"}, {Name: "c", Domain: ".other.com", Secure: true}}, func(err error) { got = err })
	app.Settle()
	if core.ErrorCode(got) != core.CodeAccessDenied || !strings.Contains(got.Error(), "https://other.com/") || len(app.WebCookies) != 1 {
		t.Fatalf("refused: %v, store %+v", got, app.WebCookies)
	}
}
