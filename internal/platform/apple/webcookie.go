//go:build darwin

package apple

import (
	"fmt"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
)

// SetWebCookies carries out an ItemWebCookies: it sets each cookie in WKWebsiteDataStore's default store, whose WKHTTPCookieStore every web view but an Incognito one reads, and answers once WebKit has taken them all. The store exists without a web view, and WebKit writes it to disk, so a cookie with an expiry outlives the app. A target without WebKit, tvOS, has no store.
func SetWebCookies(id uint32, r driver.WebCookiesRequest, dispatch func(driver.Event)) {
	if _, err := purego.Dlopen("/System/Library/Frameworks/WebKit.framework/WebKit", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		dispatch(driver.NoWebCookies(&driver.MountItem{Count: id}))
		return
	}
	loadWebKit()
	store := objc.ID(Class("WKWebsiteDataStore")).Send(sel("defaultDataStore")).Send(sel("httpCookieStore"))
	pending, failure := len(r.Cookies), ""
	finish := func() {
		if pending--; pending == 0 {
			dispatch(driver.Event{Kind: driver.EventWebCookies, Count: id, Text: failure})
		}
	}
	for _, c := range r.Cookies {
		cookie := httpCookie(c)
		if cookie == 0 {
			if failure == "" {
				failure = fmt.Sprintf("nsgo: Foundation refused the cookie %q of %s", c.Name, c.Domain)
			}
			finish()
			continue
		}
		// WebKit calls the handler on the main thread; OnMain keeps the count there whatever it does.
		set := objcblock.New(func(objc.Block) { OnMain(finish) })
		store.Send(sel("setCookie:completionHandler:"), cookie, set)
		set.Release()
	}
}

// httpCookie returns c as an autoreleased NSHTTPCookie, 0 when Foundation refuses it. A session cookie is one to discard, and SameSite None leaves the policy out, as Foundation has none for it and WebKit restricts no cookie without one.
func httpCookie(c driver.WebCookie) objc.ID {
	props := objc.ID(Class("NSMutableDictionary")).Send(sel("dictionary"))
	set := func(key string, value objc.ID) {
		if k := Global(foundationPath, key); k != 0 && value != 0 {
			props.Send(sel("setObject:forKey:"), value, k)
		}
	}
	yes := NSString("TRUE")
	set("NSHTTPCookieName", NSString(c.Name))
	set("NSHTTPCookieValue", NSString(c.Value))
	set("NSHTTPCookieDomain", NSString(c.Domain))
	set("NSHTTPCookiePath", NSString(c.CookiePath()))
	if c.Expires.IsZero() {
		set("NSHTTPCookieDiscard", yes)
	} else {
		seconds := float64(c.Expires.UnixNano()) / 1e9
		set("NSHTTPCookieExpires", objc.ID(Class("NSDate")).Send(sel("dateWithTimeIntervalSince1970:"), seconds))
	}
	if c.Secure {
		set("NSHTTPCookieSecure", yes)
	}
	if c.HTTPOnly {
		// Foundation exports no key for it; CFNetwork reads the attribute's name, as it parses Set-Cookie.
		props.Send(sel("setObject:forKey:"), yes, NSString("HttpOnly"))
	}
	switch c.SameSite {
	case driver.WebSameSiteLax:
		set("NSHTTPCookieSameSitePolicy", Global(foundationPath, "NSHTTPCookieSameSiteLax"))
	case driver.WebSameSiteStrict:
		set("NSHTTPCookieSameSitePolicy", Global(foundationPath, "NSHTTPCookieSameSiteStrict"))
	}
	return objc.ID(Class("NSHTTPCookie")).Send(sel("cookieWithProperties:"), props)
}
