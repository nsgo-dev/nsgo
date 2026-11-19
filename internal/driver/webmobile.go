package driver

import (
	_ "embed"
	"strconv"
)

//go:embed webmobile.js
var webMobileScript string

// WebMobileScript is the script a backend runs in every frame before a page's own for WebConfig.Mobile. With emulate, as WebKit's desktop backends have it, the page sees a touch screen without hover, with navigator.platform reporting platform, and the mouse's drags reach it as touches; either way wheel and trackpad scrolls that nothing would scroll reach it as swipes.
func WebMobileScript(platform string, emulate bool) string {
	return webMobileScript + "(" + JSString(platform) + ", " + strconv.FormatBool(emulate) + ");"
}

// The phones' user agents a Mobile web view presents without one of its own: Safari's on an iPhone, which WKWebView on macOS presents, and Chrome's on Android, with a Chrome major version in place of %s, which WebView2 presents with its own and WebKitGTK with WebMobileChromeVersion.
const (
	WebMobileUserAgentIPhone  = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1"
	WebMobileUserAgentAndroid = "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Mobile Safari/537.36"
)

// WebMobileChromeVersion is the Chrome major version Android Chrome's user agent names where the engine has none of its own: WebKitGTK's, and WebView2's when its own user agent names none. Sites may treat a browser many versions old as unsupported, so update it with the iPhone's user agent now and then: Chrome 157 was current in November 2026, and a new major version ships every four weeks.
const WebMobileChromeVersion = "157"
