package driver

import (
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

// WebCookiesRequest is an ItemWebCookies' request: the cookies to add or replace, those of the same name, domain and path, in the persistent store.
type WebCookiesRequest struct {
	Cookies []WebCookie
	// Schemes are the app schemes the calling scope serves, which Windows registers if the request is what starts WebView2's environment, as a web view's would.
	Schemes []string
}

// NoWebCookieStore is EventWebCookies' Text where the platform has no store to write, as on the web and where Env.HasWebView is false.
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
