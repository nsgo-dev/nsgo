//go:build linux && !android

package linux

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The cookie store's calls, bound with the rest of WebKitGTK (bindWebCookies).
var (
	webkitCookieManagerAddCookie       func(manager, cookie, cancellable, callback, data uintptr)
	webkitCookieManagerAddCookieFinish func(manager, result, gerr uintptr) int32
	soupCookieNew                      func(name, value, domain, path *byte, maxAge int32) uintptr
	soupCookieSetSecure                func(cookie uintptr, on int32)
	soupCookieSetHTTPOnly              func(cookie uintptr, on int32)
	soupCookieSetSameSitePolicy        func(cookie uintptr, policy int32)
	soupCookieFree                     func(cookie uintptr)
)

func bindWebCookies(webkit, soup uintptr) {
	purego.RegisterLibFunc(&webkitCookieManagerAddCookie, webkit, "webkit_cookie_manager_add_cookie")
	purego.RegisterLibFunc(&webkitCookieManagerAddCookieFinish, webkit, "webkit_cookie_manager_add_cookie_finish")
	purego.RegisterLibFunc(&soupCookieNew, soup, "soup_cookie_new")
	purego.RegisterLibFunc(&soupCookieSetSecure, soup, "soup_cookie_set_secure")
	purego.RegisterLibFunc(&soupCookieSetHTTPOnly, soup, "soup_cookie_set_http_only")
	purego.RegisterLibFunc(&soupCookieSetSameSitePolicy, soup, "soup_cookie_set_same_site_policy")
	purego.RegisterLibFunc(&soupCookieFree, soup, "soup_cookie_free")
}

// setWebCookies carries out an ItemWebCookies in the cookie manager of the app's network session, which every web view but an Incognito one uses and which keeps cookies in its SQLite file (webSession), so it needs no web view; it answers once WebKit has added them all.
func (w *window) setWebCookies(id uint32, r driver.WebCookiesRequest) {
	answer := func(text string) {
		w.engine.Dispatch(driver.Event{Kind: driver.EventWebCookies, Count: id, Text: text})
	}
	if err := openWebKit(); err != nil {
		answer("nsgo: " + err.Error())
		return
	}
	manager := webkitNetworkSessionGetCookieManager(w.webSession())
	if manager == 0 {
		answer("nsgo: WebKitGTK's network session has no cookie manager")
		return
	}
	pending, failure := len(r.Cookies), ""
	finish := func(text string) {
		if failure == "" {
			failure = text
		}
		if pending--; pending == 0 && !w.closing {
			answer(failure)
		}
	}
	for _, c := range r.Cookies {
		cookie := soupCookie(c)
		if cookie == 0 {
			finish(fmt.Sprintf("nsgo: libsoup refused the cookie %q of %s", c.Name, c.Domain))
			continue
		}
		webkitCookieManagerAddCookie(manager, cookie, 0, cbAsync, async(func(source, result uintptr) {
			var gerr uintptr
			if webkitCookieManagerAddCookieFinish(source, result, uintptr(unsafe.Pointer(&gerr))) != 0 {
				finish("")
				return
			}
			text := "nsgo: WebKitGTK did not add the cookie " + c.Name
			if err := dialogError(gerr); err != nil {
				text = "nsgo: " + err.Error()
			}
			finish(text)
		}))
		// WebKit copies the cookie as it is added.
		soupCookieFree(cookie)
	}
}

// SoupSameSitePolicy
const (
	soupSameSiteNone   = 0
	soupSameSiteLax    = 1
	soupSameSiteStrict = 2
)

// soupCookie returns c as a SoupCookie the caller frees, 0 when libsoup refuses it. libsoup's domain takes the leading dot of a cookie every host under it shares, as c's does, and a max age of -1 makes a session cookie.
func soupCookie(c driver.WebCookie) uintptr {
	maxAge := int32(-1)
	if !c.Expires.IsZero() {
		maxAge = int32(min(max(time.Until(c.Expires)/time.Second, 0), 1<<31-1))
	}
	cookie := soupCookieNew(cstr(c.Name), cstr(c.Value), cstr(c.Domain), cstr(c.CookiePath()), maxAge)
	if cookie == 0 {
		return 0
	}
	soupCookieSetSecure(cookie, boolInt(c.Secure))
	soupCookieSetHTTPOnly(cookie, boolInt(c.HTTPOnly))
	switch c.SameSite {
	case driver.WebSameSiteLax:
		soupCookieSetSameSitePolicy(cookie, soupSameSiteLax)
	case driver.WebSameSiteStrict:
		soupCookieSetSameSitePolicy(cookie, soupSameSiteStrict)
	case driver.WebSameSiteNone:
		soupCookieSetSameSitePolicy(cookie, soupSameSiteNone)
	}
	return cookie
}
