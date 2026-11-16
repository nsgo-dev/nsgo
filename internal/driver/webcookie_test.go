package driver

import (
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
