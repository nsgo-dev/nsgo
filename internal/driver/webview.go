package driver

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// WebSource is what a web view shows: the page at Src, or HTML, whose
// relative links resolve against BaseURL, instead when it is set.
type WebSource struct {
	Src     string
	HTML    string
	BaseURL string
}

// Origin is the origin of the page the source loads, as WebOrigin writes it: Src's, or for HTML, BaseURL's, which is "null" without one. A source with neither has none ("").
func (s WebSource) Origin() string {
	switch {
	case s.HTML != "" && s.BaseURL == "":
		return "null"
	case s.HTML != "":
		return WebOrigin(s.BaseURL)
	case s.Src != "":
		return WebOrigin(s.Src)
	}
	return ""
}

// Shows reports whether rawURL is the page s loads, which tells apart the pages of an opaque origin ("null"), as every one of them has the same: HTML without a base URL is at about:blank (about:srcdoc on the web), other HTML at BaseURL and a Src at itself, fragments aside. A page the view went to, such as a data: URL a frame sent it to, is not.
func (s WebSource) Shows(rawURL string) bool {
	page, _, _ := strings.Cut(rawURL, "#")
	switch {
	case s.HTML != "" && s.BaseURL == "":
		u, err := url.Parse(page)
		return err == nil && strings.EqualFold(u.Scheme, "about") && (u.Opaque == "blank" || u.Opaque == "srcdoc")
	case s.HTML != "":
		return sameURL(page, s.BaseURL)
	case s.Src != "":
		return sameURL(page, s.Src)
	}
	return false
}

// sameURL reports whether page and want, a source's URL, are one URL, as platforms escape a data: URL's characters their own ways.
func sameURL(page, want string) bool {
	want, _, _ = strings.Cut(want, "#")
	if page == want {
		return true
	}
	p, err1 := url.PathUnescape(page)
	w, err2 := url.PathUnescape(want)
	return err1 == nil && err2 == nil && p == w
}

// WebOrigin is the origin of the page at rawURL as browsers write one: scheme://host, with the port when it is not the scheme's own, in lower case, such as "https://example.com" or "app://bundle". A blob: URL has the origin of the URL it holds and every file: URL "file://"; a URL with no host, such as about:blank or a data: URL, or one that does not parse, has an opaque origin, written "null". No URL ("") has no origin ("").
func WebOrigin(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "null"
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "blob":
		return WebOrigin(u.Opaque)
	case "file":
		return "file://"
	case "":
		return "null"
	}
	if u.Host == "" || u.Opaque != "" {
		return "null"
	}
	host := strings.ToLower(u.Host)
	if p := u.Port(); p != "" && p == defaultPorts[scheme] {
		host = strings.TrimSuffix(host, ":"+p)
	}
	return scheme + "://" + host
}

// defaultPorts are the ports WebOrigin leaves out, by scheme.
var defaultPorts = map[string]string{"http": "80", "https": "443", "ws": "80", "wss": "443", "ftp": "21"}

// WebNavigation is a web view's page and history.
type WebNavigation struct {
	URL          string
	Title        string
	Loading      bool
	CanGoBack    bool
	CanGoForward bool
}

// WebDownload is a download of a web view: its State, and where it is
// saved. Bytes is what it wrote once finished, and Message why it failed.
type WebDownload struct {
	State    WebDownloadState
	ID       string
	URL      string
	Filename string
	Path     string
	Bytes    int64
	Message  string
}

// WebDownloadState is how far a WebDownload got.
type WebDownloadState uint8

const (
	WebDownloadStarted WebDownloadState = iota
	WebDownloadFinished
	WebDownloadFailed
)

// WebFailure identifies the platform error behind an EventError. Domain is
// empty for an error the engine produced itself, such as a refused
// navigation.
type WebFailure struct {
	Domain string
	Code   int
}

// WebConfig is everything a web view loads and allows (ItemWeb).
// Platforms load the source again when it or the scripts change.
type WebConfig struct {
	Source                          WebSource
	InjectedJavaScript              string
	InjectedJavaScriptBeforeContent string
	UserAgent                       string
	// Headers are sent with the initial URL request where the platform
	// supports custom request headers. HTML sources do not make one.
	Headers map[string]string
	// Incognito keeps website data in memory for this view. Platforms read
	// this when the native view is first made.
	Incognito bool
	// SchemeHandlers are app-owned URL schemes and their response bodies.
	// Platforms read them when the native view is first made.
	SchemeHandlers map[string]func(string) []byte
	NoScroll       bool
	NoJavaScript   bool
	// PinchZoom enables the platform web view's pinch magnification where it
	// has a setting for it. Platforms without one keep their native behavior.
	PinchZoom           bool
	BackForwardGestures bool
	// DownloadDir is where the view saves what its pages download, a
	// directory of its own for each (<id>/<file name>); "" saves nothing.
	DownloadDir string
	// Restricted has the platform refuse the top-level navigations
	// Navigable refuses, as they start, reporting EventError.
	Restricted bool
	// Hosts are the hosts Navigable allows, space-separated patterns:
	// "*", "*.example.com" or a host.
	Hosts string
	// Media is what the pages may capture through getUserMedia; the
	// platform refuses the rest as the page asks.
	Media WebMedia
}

// WebMedia is a set of the devices a web view's pages may capture.
type WebMedia uint8

const (
	WebMicrophone WebMedia = 1 << iota
	WebCamera
)

// Allows reports whether m holds every device of want, so a page asking
// for both gets neither unless both are allowed.
func (m WebMedia) Allows(want WebMedia) bool { return want != 0 && m&want == want }

// Navigable reports whether a restricted view's page may navigate to
// rawURL at its top level: a page of its own (about:, data:, blob:), or
// an http or https one of a host Hosts allows. Other schemes, file:
// among them, are refused.
func (c *WebConfig) Navigable(rawURL string) bool {
	if !c.Restricted {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "about", "data", "blob":
		return true
	case "http", "https":
		return AllowsHost(strings.Fields(c.Hosts), u.Hostname())
	}
	if _, ok := c.SchemeHandlers[strings.ToLower(u.Scheme)]; ok {
		return true
	}
	return false
}

// RefusedNavigation is the message of EventError for a navigation
// Navigable refused.
func RefusedNavigation(url string) string {
	return "nsgo: the web view may not navigate to " + url + " (the webview capability's hosts)"
}

// AllowsHost reports whether one of patterns allows host: "*" any, "*.example.com"
// its subdomains, else the host itself. Hosts compare in lower case.
func AllowsHost(patterns []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, p := range patterns {
		if HostCovers(p, host) {
			return true
		}
	}
	return false
}

// HostCovers reports whether pattern covers other, a host or a pattern.
func HostCovers(pattern, other string) bool {
	pattern, other = strings.ToLower(pattern), strings.ToLower(other)
	if pattern == "*" {
		return true
	}
	if suffix, ok := strings.CutPrefix(pattern, "*."); ok {
		if inner, ok := strings.CutPrefix(other, "*."); ok && inner == suffix {
			return true
		}
		return strings.HasSuffix(other, "."+suffix)
	}
	return pattern == other
}

// IntersectHosts returns the patterns allowing what both a and b allow.
func IntersectHosts(a, b []string) []string {
	var out []string
	for _, x := range a {
		for _, y := range b {
			switch {
			case HostCovers(x, y):
				out = append(out, y)
			case HostCovers(y, x):
				out = append(out, x)
			}
		}
	}
	return out
}

// WebCommand is what ItemWebCommand asks a web view to do.
type WebCommand uint8

const (
	WebReload WebCommand = iota + 1
	WebGoBack
	WebGoForward
	WebStopLoading
	// WebPostMessage dispatches a MessageEvent carrying Value on the
	// page's window.
	WebPostMessage
	// WebInjectJavaScript runs Value in the page.
	WebInjectJavaScript
	// WebSendInput gives the page Data, a *WebInput, as the user's own
	// input.
	WebSendInput
	// WebLoad loads the WebSource in Data, even when it equals the current
	// source. The current WebConfig's request headers still apply.
	WebLoad
)

// WebInput is input a web view's page takes as the user's hands give it:
// its events are trusted (isTrusted), which no script it runs can make.
type WebInput struct {
	Kind WebInputKind
	// Point is a click's, in CSS pixels from the viewport's top left, as
	// getBoundingClientRect reports.
	Point Point
	// Text is what WebInputText types.
	Text string
	// Key is what WebInputKey presses, as KeyboardEvent.key names it:
	// Enter, Tab, Escape or Backspace.
	Key string
}

// WebInputKind is what a WebInput gives.
type WebInputKind uint8

const (
	// WebInputClick is a primary click at Point.
	WebInputClick WebInputKind = iota
	// WebInputText types Text into what has the page's focus.
	WebInputText
	// WebInputKey presses and releases Key.
	WebInputKey
)

// WebBridge is the script a platform runs at the start of every page
// before the app's, given post, the JavaScript expression of a function
// that sends its string argument to the app.
func WebBridge(post string) string {
	return "window.nsgo = window.nsgo || { postMessage: function (data) { (" + post + ")(String(data)); } };"
}

// WebEvaluateScript is the script that runs script in a page as an
// expression and posts its answer through the bridge, marker first: its
// result as JSON, a promise's once settled, or what it threw (decoded by
// WebEvaluateAnswer).
func WebEvaluateScript(script, marker string) string {
	return webEvaluateHead + JSString(marker) + ` + JSON.stringify(answer)); };
  var fail = function (e) { send({ error: String(e && e.message !== undefined ? e.message : e) }); };
  try {
    Promise.resolve((0, eval)(` + JSString(script) + `)).then(function (v) {
      var json;
      try { json = JSON.stringify(v === undefined ? null : v); } catch (e) { fail(e); return; }
      if (json === undefined) { fail("the result is no JSON value"); return; }
      send({ json: json });
    }, fail);
  } catch (e) { fail(e); }
})();`
}

// webEvaluateHead starts every WebEvaluateScript, up to its marker.
const webEvaluateHead = `(function () {
  var bridge = window.nsgo, post = bridge && bridge.postMessage;
  if (!post) return;
  var send = function (answer) { post.call(bridge, `

// WebEvaluateMarker returns the marker of a script WebEvaluateScript made, for a backend that has no page to run it in and answers it with WebEvaluateFailure instead.
func WebEvaluateMarker(script string) (string, bool) {
	rest, ok := strings.CutPrefix(script, webEvaluateHead)
	if !ok || !strings.HasPrefix(rest, `"`) {
		return "", false
	}
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case '\\':
			i++
		case '"':
			// JSString's literals are JSON's.
			var marker string
			if json.Unmarshal([]byte(rest[:i+1]), &marker) != nil {
				return "", false
			}
			return marker, true
		}
	}
	return "", false
}

// WebEvaluateFailure is what a page posts when the script after marker threw message, as WebEvaluateAnswer decodes it.
func WebEvaluateFailure(marker, message string) string {
	answer, _ := json.Marshal(map[string]string{"error": message})
	return marker + string(answer)
}

// WebEvaluateAnswer decodes what WebEvaluateScript posted after its
// marker: the result as JSON, or the error the script threw.
func WebEvaluateAnswer(answer string) (string, error) {
	var a struct {
		JSON  *string `json:"json"`
		Error *string `json:"error"`
	}
	if err := json.Unmarshal([]byte(answer), &a); err != nil {
		return "", err
	}
	switch {
	case a.Error != nil:
		return "", errors.New(*a.Error)
	case a.JSON == nil:
		return "", errors.New("nsgo: a web view's answer holds no result")
	}
	return *a.JSON, nil
}

// WebMessageScript is the script WebPostMessage runs in the page.
func WebMessageScript(data string) string {
	return "window.dispatchEvent(new MessageEvent('message', { data: " + JSString(data) + " }));"
}

// JSString quotes s as a JavaScript string literal.
func JSString(s string) string {
	b := make([]byte, 0, len(s)+2)
	b = append(b, '"')
	for _, r := range s {
		switch {
		case r == '"':
			b = append(b, '\\', '"')
		case r == '\\':
			b = append(b, '\\', '\\')
		case r == '\n':
			b = append(b, '\\', 'n')
		case r == '\r':
			b = append(b, '\\', 'r')
		case r < 0x20 || r == 0x2028 || r == 0x2029:
			const hex = "0123456789abcdef"
			b = append(b, '\\', 'u', hex[r>>12&15], hex[r>>8&15], hex[r>>4&15], hex[r&15])
		default:
			b = append(b, string(r)...)
		}
	}
	return string(append(b, '"'))
}
