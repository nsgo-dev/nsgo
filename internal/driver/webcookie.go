package driver

import (
	"cmp"
	"net/url"
	"slices"
	"strings"
	"time"
)

// WebCookie is a cookie of the store the app's web views share, as core.WebCookie.
type WebCookie struct {
	Name, Value string
	// Domain is the host the cookie goes to, and with a leading dot (".example.com") every host under it too, as a Set-Cookie Domain attribute makes it.
	Domain string
	// Path is the path the cookie goes to, and what is under it; "" is "/".
	Path string
	// Expires is when the store drops the cookie, which Apple platforms, Windows and Android bring to at most 400 days on; zero for a session cookie, which goes as the app ends.
	Expires          time.Time
	Secure, HTTPOnly bool
	SameSite         WebSameSite
}

// WebSameSite is a cookie's SameSite attribute.
type WebSameSite uint8

const (
	// WebSameSiteDefault leaves the attribute out, so the platform's default for a cookie without one applies, which differs: Chromium's web views, on Windows and Android, take it as Lax.
	WebSameSiteDefault WebSameSite = iota
	WebSameSiteLax
	WebSameSiteStrict
	// WebSameSiteNone sends the cookie with requests from other sites too, which every platform allows only for a Secure cookie.
	WebSameSiteNone
)

// WebCookiesRequest is an ItemWebCookies' request of the persistent store: Op says what it does with it.
type WebCookiesRequest struct {
	Op WebCookiesOp
	// Cookies are what WebCookiesSet adds, replacing those of the same name, domain and path.
	Cookies []WebCookie
	// Filter picks the cookies WebCookiesList answers with.
	Filter WebCookieFilter
	// Schemes are the app schemes the calling scope serves, which Windows registers if the request is what starts WebView2's environment, as a web view's would.
	Schemes []string
}

// WebCookiesOp is what an ItemWebCookies does with the store.
type WebCookiesOp uint8

const (
	// WebCookiesSet adds or replaces the request's Cookies; the answer carries no Data.
	WebCookiesSet WebCookiesOp = iota
	// WebCookiesList answers with the cookies Filter picks, HttpOnly ones included, as the event's Data, a []WebCookie.
	WebCookiesList
	// WebCookiesClear removes everything the store holds: cookies, local and session storage, IndexedDB, service workers and caches.
	WebCookiesClear
)

// WebCookieFilter picks cookies of the store, as core.WebCookieFilter: those a request to URL would send, or those of Domain and every host under it; the zero filter picks all. Core sends at most one of them, URL an http or https URL with a host and Domain a host in lower case without a leading dot.
type WebCookieFilter struct {
	URL, Domain string
}

// NoWebCookieDomains is EventWebCookies' Text where the store lists cookies only by URL, as Android's CookieManager does.
const NoWebCookieDomains = "nsgo: this platform lists web view cookies only by URL"

// FilterWebCookies returns the cookies of cookies that f picks and that have not expired at now, in the order of their host, path and name: those a platform lists whole, as Apple's, Windows' and Linux's stores do, go through it.
func FilterWebCookies(cookies []WebCookie, f WebCookieFilter, now time.Time) []WebCookie {
	picks := func(WebCookie) bool { return true }
	switch {
	case f.URL != "":
		u, err := url.Parse(f.URL)
		if err != nil {
			return nil
		}
		host, path := strings.ToLower(u.Hostname()), u.EscapedPath()
		if path == "" {
			path = "/"
		}
		secure := u.Scheme == "https" || u.Scheme == "wss"
		picks = func(c WebCookie) bool {
			return (!c.Secure || secure) && c.goesTo(host) && pathMatches(path, c.CookiePath())
		}
	case f.Domain != "":
		picks = func(c WebCookie) bool { return c.Host() == f.Domain || strings.HasSuffix(c.Host(), "."+f.Domain) }
	}
	out := []WebCookie{}
	for _, c := range cookies {
		if (c.Expires.IsZero() || c.Expires.After(now)) && picks(c) {
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b WebCookie) int {
		return cmp.Or(strings.Compare(a.Host(), b.Host()), strings.Compare(a.CookiePath(), b.CookiePath()), strings.Compare(a.Name, b.Name))
	})
	return out
}

// goesTo reports whether c goes with a request to host, as RFC 6265 matches a domain: its host alone, or a host-only one's, and the hosts under it too for one with a leading dot.
func (c WebCookie) goesTo(host string) bool {
	h := c.Host()
	return host == h || !c.HostOnly() && strings.HasSuffix(host, "."+h)
}

// pathMatches reports whether a request's path takes a cookie's, as RFC 6265 matches paths: the same path, or one under it.
func pathMatches(path, cookie string) bool {
	if !strings.HasPrefix(path, cookie) {
		return false
	}
	return len(path) == len(cookie) || strings.HasSuffix(cookie, "/") || path[len(cookie)] == '/'
}

// NoWebCookieStore is EventWebCookies' Text where the platform has no store, as on the web and where Env.HasWebView is false.
const NoWebCookieStore = "nsgo: this platform has no web view cookie store"

// NoWebCookies is the answer of a platform without a store to it.
func NoWebCookies(it *MountItem) Event {
	return Event{Kind: EventWebCookies, Count: it.Count, Text: NoWebCookieStore}
}

// Host is the host c goes to, its Domain without the leading dot, in lower case.
func (c WebCookie) Host() string { return strings.ToLower(strings.TrimPrefix(c.Domain, ".")) }

// HostOnly reports whether c goes only to its host, not the hosts under it.
func (c WebCookie) HostOnly() bool { return !strings.HasPrefix(c.Domain, ".") }

// CookiePath is c's Path, "/" when it has none.
func (c WebCookie) CookiePath() string {
	if c.Path == "" {
		return "/"
	}
	return c.Path
}

// URL is a URL c goes with: https for a Secure cookie, else http, at its host and path. Android's CookieManager takes a cookie for one, and Access is asked for it.
func (c WebCookie) URL() string {
	scheme := "http"
	if c.Secure {
		scheme = "https"
	}
	host := c.Host()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return scheme + "://" + host + c.CookiePath()
}

// SetCookie is c as a Set-Cookie header's value writes it, the attributes in the order browsers write them: Android's CookieManager reads one. The domain goes only for a cookie its hosts under share, as a host-only cookie has none; a session cookie has no Expires.
func (c WebCookie) SetCookie() string {
	var b strings.Builder
	b.WriteString(c.Name)
	b.WriteByte('=')
	b.WriteString(c.Value)
	if !c.HostOnly() {
		b.WriteString("; Domain=")
		b.WriteString(c.Host())
	}
	b.WriteString("; Path=")
	b.WriteString(c.CookiePath())
	if !c.Expires.IsZero() {
		b.WriteString("; Expires=")
		b.WriteString(c.Expires.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	}
	if c.Secure {
		b.WriteString("; Secure")
	}
	if c.HTTPOnly {
		b.WriteString("; HttpOnly")
	}
	switch c.SameSite {
	case WebSameSiteLax:
		b.WriteString("; SameSite=Lax")
	case WebSameSiteStrict:
		b.WriteString("; SameSite=Strict")
	case WebSameSiteNone:
		b.WriteString("; SameSite=None")
	}
	return b.String()
}
