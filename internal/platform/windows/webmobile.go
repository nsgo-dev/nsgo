//go:build windows

package windows

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// WebView.Mobile on Windows is Chrome's device mode, through the DevTools protocol: Android Chrome's user agent and client hints, a touch screen with five points whose media queries match pointer: coarse and hover: none, the mouse's presses and drags as touches, which scroll and fling natively, and the phone's viewport, as large as the view. The page's session keeps them across navigations. What WebView2 leaves a wheel, a trackpad's swipe over what nothing would scroll, the document script makes a swipe.

// chromeVersion finds the major version in WebView2's own user agent.
var chromeVersion = regexp.MustCompile(`Chrome/(\d+)`)

// mobileUserAgent is Android Chrome's user agent, of the version of WebView2's own, ua.
func mobileUserAgent(ua string) string {
	return fmt.Sprintf(driver.WebMobileUserAgentAndroid, majorVersion(ua))
}

func majorVersion(ua string) string {
	if m := chromeVersion.FindStringSubmatch(ua); m != nil {
		return m[1]
	}
	return driver.WebMobileChromeVersion
}

type cdpBrand struct {
	Brand   string `json:"brand"`
	Version string `json:"version"`
}

// emulatePhone has the page's session present a phone, or stop for a config without Mobile. The navigation waits for the commands, as it does for the document scripts.
func (wv *webView) emulatePhone(c driver.WebConfig) {
	on := c.Mobile
	ua := c.UserAgent
	if ua == "" {
		ua = mobileUserAgent(wv.userAgent)
	}
	major := majorVersion(wv.userAgent)
	brands := []cdpBrand{{"Chromium", major}, {"Not_A Brand", "24"}}
	// An empty user agent ends the override, leaving the settings' own.
	agent := map[string]any{"userAgent": ""}
	if on {
		agent = map[string]any{"userAgent": ua, "platform": "Linux armv8l", "userAgentMetadata": map[string]any{
			"brands": brands, "fullVersion": major + ".0.0.0", "platform": "Android", "platformVersion": "10.0.0",
			"architecture": "", "model": "", "mobile": true}}
	}
	params, _ := json.Marshal(agent)
	wv.emulate("Emulation.setUserAgentOverride", string(params))
	wv.emulate("Emulation.setTouchEmulationEnabled", fmt.Sprintf(`{"enabled":%t,"maxTouchPoints":5}`, on))
	wv.emulate("Emulation.setEmitTouchEventsForMouse", fmt.Sprintf(`{"enabled":%t,"configuration":"mobile"}`, on))
	if on {
		wv.emulate("Emulation.setDeviceMetricsOverride", `{"width":0,"height":0,"deviceScaleFactor":0,"mobile":true}`)
	} else {
		wv.emulate("Emulation.clearDeviceMetricsOverride", "{}")
	}
}

// emulate calls a DevTools method, which a pending navigation waits for.
func (wv *webView) emulate(method, params string) {
	wv.adding++
	wv.devTools(method, params, "present a phone", func(_ string, err error) {
		wv.adding--
		if err != nil {
			wv.report(driver.Event{Kind: driver.EventError, Text: err.Error()})
		}
		wv.navigateIfReady()
	})
}
