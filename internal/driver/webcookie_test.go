package driver

import (
	"strings"
	"testing"
	"time"
)

func TestWebCookieSetCookieAndURL(t *testing.T) {
	expires := time.Date(2030, 1, 2, 3, 4, 5, 0, time.FixedZone("JST", 9*3600))
	for _, c := range []struct {
		cookie         WebCookie
		url, setCookie string
	}{
		{WebCookie{Name: "SID", Value: "a b", Domain: ".example.com", Expires: expires, Secure: true, HTTPOnly: true, SameSite: WebSameSiteNone},
			"https://example.com/", "SID=a b; Domain=example.com; Path=/; Expires=Tue, 01 Jan 2030 18:04:05 GMT; Secure; HttpOnly; SameSite=None"},
		{WebCookie{Name: "pref", Value: "1", Domain: "www.example.com", Path: "/watch", SameSite: WebSameSiteLax},
			"http://www.example.com/watch", "pref=1; Path=/watch; SameSite=Lax"},
		{WebCookie{Name: "a", Domain: "::1", SameSite: WebSameSiteStrict}, "http://[::1]/", "a=; Path=/; SameSite=Strict"},
	} {
		if got := c.cookie.URL(); got != c.url {
			t.Errorf("%+v: URL %q, want %q", c.cookie, got, c.url)
		}
		if got := c.cookie.SetCookie(); got != c.setCookie {
			t.Errorf("%+v: SetCookie %q, want %q", c.cookie, got, c.setCookie)
		}
	}
}

func TestFilterWebCookiesPicksWhatARequestSends(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	store := []WebCookie{
		{Name: "s", Value: "1", Domain: ".example.test", Secure: true, HTTPOnly: true},
		{Name: "host", Value: "2", Domain: "example.test"},
		{Name: "www", Value: "3", Domain: "www.example.test", Path: "/watch"},
		{Name: "deep", Value: "4", Domain: "a.www.example.test"},
		{Name: "other", Value: "5", Domain: ".other.test"},
		{Name: "gone", Value: "6", Domain: ".example.test", Expires: now.Add(-time.Second)},
		{Name: "kept", Value: "7", Domain: ".example.test", Expires: now.Add(time.Hour)},
		{Name: "lookalike", Value: "8", Domain: "notexample.test"},
	}
	names := func(f WebCookieFilter) string {
		var got []string
		for _, c := range FilterWebCookies(store, f, now) {
			got = append(got, c.Name)
		}
		return strings.Join(got, " ")
	}
	for _, c := range []struct {
		filter WebCookieFilter
		want   string
	}{
		{WebCookieFilter{}, "deep host kept s lookalike other www"},
		{WebCookieFilter{Domain: "example.test"}, "deep host kept s www"},
		{WebCookieFilter{Domain: "www.example.test"}, "deep www"},
		{WebCookieFilter{Domain: "other.test"}, "other"},
		{WebCookieFilter{URL: "https://example.test/"}, "host kept s"},
		{WebCookieFilter{URL: "http://example.test/"}, "host kept"},
		{WebCookieFilter{URL: "https://www.example.test/watch/1?x=y"}, "kept s www"},
		{WebCookieFilter{URL: "https://www.example.test/watching"}, "kept s"},
		{WebCookieFilter{URL: "https://www.example.test"}, "kept s"},
		{WebCookieFilter{URL: "https://a.www.example.test/"}, "deep kept s"},
		{WebCookieFilter{URL: "https://other.test/"}, "other"},
	} {
		if got := names(c.filter); got != c.want {
			t.Errorf("%+v picks %q, want %q", c.filter, got, c.want)
		}
	}
	if got := FilterWebCookies(nil, WebCookieFilter{}, now); got == nil || len(got) != 0 {
		t.Errorf("an empty store answers %#v, want an empty list", got)
	}
}
