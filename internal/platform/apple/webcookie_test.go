//go:build darwin

package apple

import (
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestHTTPCookieCarriesEveryAttribute builds the NSHTTPCookie SetWebCookies hands WebKit and reads its attributes back, HttpOnly, which Foundation has no key for, among them.
func TestHTTPCookieCarriesEveryAttribute(t *testing.T) {
	pool := objc.ID(Class("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	// Foundation keeps a cookie at most 400 days, as RFC 6265bis says.
	expires := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	for _, c := range []struct {
		cookie   driver.WebCookie
		session  bool
		sameSite string
	}{
		{driver.WebCookie{Name: "SID", Value: "v", Domain: ".example.com", Path: "/watch", Expires: expires, Secure: true, HTTPOnly: true, SameSite: driver.WebSameSiteStrict}, false, "strict"},
		{driver.WebCookie{Name: "pref", Value: "1", Domain: "www.example.com", SameSite: driver.WebSameSiteLax}, true, "lax"},
		{driver.WebCookie{Name: "x", Value: "2", Domain: "example.com", Secure: true, SameSite: driver.WebSameSiteNone}, true, ""},
	} {
		cookie := httpCookie(c.cookie)
		if cookie == 0 {
			t.Fatalf("%+v: Foundation refused it", c.cookie)
		}
		str := func(name string) string { return GoString(cookie.Send(sel(name))) }
		flag := func(name string) bool { return objc.Send[bool](cookie, sel(name)) }
		if str("name") != c.cookie.Name || str("value") != c.cookie.Value || str("domain") != c.cookie.Domain || str("path") != c.cookie.CookiePath() {
			t.Errorf("%+v: NSHTTPCookie %s=%s for %s%s", c.cookie, str("name"), str("value"), str("domain"), str("path"))
		}
		if flag("isSecure") != c.cookie.Secure || flag("isHTTPOnly") != c.cookie.HTTPOnly || flag("isSessionOnly") != c.session {
			t.Errorf("%+v: secure %v, HttpOnly %v, session %v", c.cookie, flag("isSecure"), flag("isHTTPOnly"), flag("isSessionOnly"))
		}
		if got := str("sameSitePolicy"); got != c.sameSite {
			t.Errorf("%+v: SameSite %q, want %q", c.cookie, got, c.sameSite)
		}
		if !c.session {
			at := objc.Send[float64](cookie.Send(sel("expiresDate")), sel("timeIntervalSince1970"))
			if int64(at) != expires.Unix() {
				t.Errorf("%+v: expires at %v", c.cookie, at)
			}
		}
	}
}
