//go:build js && wasm

package web

import (
	"cmp"
	"math/rand/v2"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// webFrame is a web view's <iframe>, as NSH's web backend has it. HTML loads as srcdoc in a sandbox with an origin of its own, so it never reaches the app's page, with a bridge script first: it hands the app a MessagePort with token, which only the srcdoc's document knows, and window.nsgo.postMessage writes to the port, which also runs injected scripts and history steps. The app takes the port only from the frame's own window, so neither a frame within the page nor a page it went to has the bridge, and the page's messages report the source's origin. A URL loads as it is (without its origin when it is the app's own); the browser hides what such a page does, so it only gets PostMessage and load events with the URL it was given.
type webFrame struct {
	frame  js.Value
	config driver.WebConfig
	source driver.WebSource // what the frame loaded last: the config's, or Ref.Load's
	loaded bool             // configured at least once
	token  string
	port   js.Value // the app's end of the bridge of the source's page, while that page shows
	portFn js.Func
	// bridging is set while the source's page loads, which hands its port; queued are the commands for it until then.
	bridging bool
	queued   []map[string]any
	url      string
	title    string
	warned   bool // that commands do not reach the page
}

// webBridge is the script an HTML source starts with: the port it hands the app queues what the page posts until the app takes it. As the page leaves, for a link or a script setting location, it posts null, which postMessage never does, so the app drops the port, whose other end goes with the page.
func webBridge(token string) string {
	return `(() => {
  const channel = new MessageChannel(), port = channel.port1;
  window.nsgo = { postMessage(data) { port.postMessage(String(data)); } };
  port.onmessage = (m) => {
    const d = m.data;
    if (d && d.eval != null) { try { (0, eval)(String(d.eval)); } catch (err) { console.error(err); } }
    else if (d && d.go) history.go(d.go);
  };
  addEventListener("pagehide", (e) => { if (!e.persisted) port.postMessage(null); });
  parent.postMessage(` + strconv.Quote(token) + `, "*", [channel.port2]);
})();`
}

func (s *surface) createWeb(v *view) {
	w := &webFrame{frame: s.element("iframe")}
	v.web = w
	w.frame.Call("setAttribute", "allowtransparency", "true")
	setStyle(w.frame, map[string]string{
		"width": "100%", "height": "100%", "border": "0", "background": "transparent", "border-radius": "inherit",
	})
	v.inner = w.frame
	v.el.Call("appendChild", w.frame)
	v.listeners.on(w.frame, "load", func(js.Value) { s.webLoaded(v) })
	v.listeners.on(js.Global(), "message", func(e js.Value) { s.webBridged(v, e) })
}

// webBridged takes the port an HTML page's bridge hands the app: from the frame's own window, with the page's token.
func (s *surface) webBridged(v *view, e js.Value) {
	w := v.web
	port := e.Get("ports").Index(0)
	if w.token == "" || !e.Get("source").Equal(w.frame.Get("contentWindow")) || e.Get("data").Type() != js.TypeString ||
		e.Get("data").String() != w.token || !port.Truthy() {
		return
	}
	e.Call("stopImmediatePropagation")
	w.closePort()
	w.port = port
	// The page is the srcdoc, at its base URL when it has one.
	origin, page := w.source.Origin(), cmp.Or(w.source.BaseURL, "about:srcdoc")
	w.portFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		data := args[0].Get("data")
		if data.Type() != js.TypeString {
			// The page left (webBridge): what the app sends it now would be lost.
			w.closePort()
			s.webNavigation(v, driver.EventLoadStart, true)
			return nil
		}
		s.send(v, driver.Event{Kind: driver.EventMessage, Text: data.String(), Origin: origin, URL: page, MainFrame: true})
		return nil
	})
	w.port.Set("onmessage", w.portFn)
	w.bridging = false
	for _, msg := range w.queued {
		w.port.Call("postMessage", msg)
	}
	w.queued = nil
}

// sameFrame reports whether a and b load the same frame, comparing what
// NSH's web backend does: the user agent and swipes cannot apply to a
// frame.
func sameFrame(a, b *driver.WebConfig) bool {
	return a.Source == b.Source && a.InjectedJavaScript == b.InjectedJavaScript &&
		a.InjectedJavaScriptBeforeContent == b.InjectedJavaScriptBeforeContent &&
		a.NoScroll == b.NoScroll && a.NoJavaScript == b.NoJavaScript && a.Media == b.Media
}

func (s *surface) setWeb(v *view, c *driver.WebConfig) {
	w := v.web
	if w.loaded && sameFrame(c, &w.config) {
		return
	}
	w.loaded, w.config = true, *c
	s.loadFrame(v, c.Source)
}

// loadFrame loads src in v's frame with the config's settings.
func (s *surface) loadFrame(v *view, src driver.WebSource) {
	w := v.web
	c := &w.config
	w.source = src
	w.token = ""
	if src.HTML != "" && !c.NoJavaScript {
		w.token = strconv.FormatUint(rand.Uint64(), 36)
	}
	w.awaitBridge()
	sandbox := []string{"allow-forms", "allow-popups", "allow-popups-to-escape-sandbox", "allow-modals", "allow-downloads"}
	if !c.NoJavaScript {
		sandbox = append(sandbox, "allow-scripts")
	}
	// about:blank inherits the embedding page's origin unless the sandbox
	// keeps it opaque, just as an empty srcdoc page is kept opaque.
	if src.HTML == "" && src.Src != "" && src.Src != "about:blank" {
		// Pages of other sites keep their origin, so their storage works;
		// one of the app's own must not reach the app.
		url := js.Global().Get("URL")
		same := false
		if u, ok := newURL(url, src.Src); ok {
			same = u.Get("origin").String() == js.Global().Get("location").Get("origin").String()
		}
		if !same {
			sandbox = append(sandbox, "allow-same-origin")
		}
	}
	w.frame.Call("setAttribute", "sandbox", strings.Join(sandbox, " "))
	// The browser asks the user for what the frame may capture and
	// refuses the rest, from the frame's next page on.
	var allow []string
	if c.Media&driver.WebMicrophone != 0 {
		allow = append(allow, "microphone")
	}
	if c.Media&driver.WebCamera != 0 {
		allow = append(allow, "camera")
	}
	w.frame.Call("setAttribute", "allow", strings.Join(allow, "; "))
	script := func(js string) string {
		return "<script>" + strings.ReplaceAll(js, "</script", `<\/script`) + "</script>"
	}
	if src.HTML != "" {
		var head strings.Builder
		if src.BaseURL != "" {
			head.WriteString(`<base href="` + strings.ReplaceAll(src.BaseURL, `"`, "&quot;") + `">`)
		}
		if w.token != "" {
			head.WriteString(script(webBridge(w.token)))
		}
		if c.NoScroll {
			head.WriteString("<style>html,body{overflow:hidden!important}</style>")
		}
		if c.InjectedJavaScriptBeforeContent != "" {
			head.WriteString(script(c.InjectedJavaScriptBeforeContent))
		}
		tail := ""
		if c.InjectedJavaScript != "" {
			tail = script(c.InjectedJavaScript)
		}
		// After a doctype, which must stay first.
		html, doctype := src.HTML, ""
		if trimmed := strings.TrimLeft(html, " \t\r\n"); len(trimmed) > 9 && strings.EqualFold(trimmed[:9], "<!doctype") {
			if end := strings.IndexByte(trimmed, '>'); end >= 0 {
				doctype, html = trimmed[:end+1], trimmed[end+1:]
			}
		}
		w.url = src.BaseURL
		if w.url == "" {
			w.url = "about:srcdoc"
		}
		doc := js.Global().Get("DOMParser").New().Call("parseFromString", src.HTML, "text/html")
		w.title = doc.Get("title").String()
		w.frame.Call("removeAttribute", "src")
		w.frame.Call("removeAttribute", "scrolling")
		w.frame.Set("srcdoc", doctype+head.String()+html+tail)
	} else {
		w.url, w.title = src.Src, ""
		w.frame.Call("removeAttribute", "srcdoc")
		if c.NoScroll {
			w.frame.Call("setAttribute", "scrolling", "no")
		} else {
			w.frame.Call("removeAttribute", "scrolling")
		}
		if src.Src != "" {
			w.frame.Set("src", src.Src)
		} else {
			w.frame.Call("removeAttribute", "src")
		}
	}
	if src.HTML != "" || src.Src != "" {
		// After the batch, which may carry the view's events after its
		// configuration.
		var start js.Func
		start = js.FuncOf(func(js.Value, []js.Value) any {
			start.Release()
			if s.view(v.id) == v {
				s.webNavigation(v, driver.EventLoadStart, true)
			}
			return nil
		})
		js.Global().Call("queueMicrotask", start)
	}
}

func newURL(ctor js.Value, s string) (u js.Value, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return ctor.New(s, js.Global().Get("location").Get("href")), true
}

// webNavigation reports the frame's state, as far as the browser shows
// it, with kind and as a navigation change.
func (s *surface) webNavigation(v *view, kind driver.EventKind, loading bool) {
	nav := driver.WebNavigation{URL: v.web.url, Title: v.web.title, Loading: loading}
	for _, k := range []driver.EventKind{kind, driver.EventNavigationStateChange} {
		n := nav
		s.send(v, driver.Event{Kind: k, Navigation: &n})
	}
}

// webLoaded reports a loaded page.
func (s *surface) webLoaded(v *view) {
	w := v.web
	if !w.frame.Call("hasAttribute", "src").Bool() && !w.frame.Call("hasAttribute", "srcdoc").Bool() {
		return
	}
	s.webNavigation(v, driver.EventLoad, false)
}

func (w *webFrame) closePort() {
	if w.port.Truthy() {
		w.port.Call("close")
		w.port.Set("onmessage", js.Null())
		w.portFn.Release()
		w.port = js.Undefined()
	}
}

// awaitBridge drops the bridge of the page the frame shows, which is going, and waits for the next page's when the source has one.
func (w *webFrame) awaitBridge() {
	w.closePort()
	w.bridging, w.queued = w.token != "", nil
}

func (s *surface) webCommand(v *view, c driver.WebCommand, value string, data any) {
	w := v.web
	// viaPort hands msg to the source's page, once it handed its port, and reports false when no such page shows: the source is a URL, or its page went to another.
	viaPort := func(msg map[string]any) bool {
		switch {
		case w.port.Truthy():
			w.port.Call("postMessage", msg)
		case w.bridging:
			w.queued = append(w.queued, msg)
		default:
			return false
		}
		return true
	}
	warn := func(name string) {
		if !w.warned {
			w.warned = true
			js.Global().Get("console").Call("warn", "nsgo: web view: "+name+
				" only reaches the page of an HTML source (a URL's page, or one the source's went to, is the browser's own)")
		}
	}
	switch c {
	case driver.WebReload:
		// The frame is another origin: it loads again rather than is told to.
		if w.frame.Call("hasAttribute", "srcdoc").Bool() {
			w.awaitBridge()
			w.frame.Set("srcdoc", w.frame.Get("srcdoc"))
		} else if src := w.frame.Call("getAttribute", "src"); src.Truthy() {
			w.frame.Set("src", src)
		}
		s.webNavigation(v, driver.EventLoadStart, true)
	case driver.WebPostMessage:
		if win := w.frame.Get("contentWindow"); win.Truthy() {
			win.Call("postMessage", value, "*")
		}
	case driver.WebInjectJavaScript:
		if viaPort(map[string]any{"eval": value}) {
			break
		}
		marker, ok := driver.WebEvaluateMarker(value)
		if !ok {
			warn("InjectJavaScript")
			break
		}
		// No page answers an Evaluate: the view does, after the batch, as a page whose script threw.
		answer := driver.Event{Kind: driver.EventMessage, Text: driver.WebEvaluateFailure(marker,
			"nsgo: the web view shows no page of an HTML source, which alone runs scripts on the web")}
		var fail js.Func
		fail = js.FuncOf(func(js.Value, []js.Value) any {
			fail.Release()
			s.send(v, answer)
			return nil
		})
		js.Global().Call("queueMicrotask", fail)
	case driver.WebGoBack:
		if !viaPort(map[string]any{"go": -1}) {
			warn("GoBack")
		}
	case driver.WebGoForward:
		if !viaPort(map[string]any{"go": 1}) {
			warn("GoForward")
		}
	case driver.WebLoad:
		if src, ok := data.(*driver.WebSource); ok && src != nil && w.loaded {
			s.loadFrame(v, *src)
		}
	}
	// StopLoading does nothing, as in NSH: the frame is another origin.
}
