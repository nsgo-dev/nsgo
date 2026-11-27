package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the packaged loader in both standalone and embedded pages. The
// same Chrome context stays open across navigations, but the Go store does not.
func TestWebStoragePersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a web app")
	}
	if _, err := findBrowser(); err != nil {
		t.Skip(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	app, site := t.TempDir(), t.TempDir()
	writeFiles(t, app, map[string]string{
		"go.mod":    "module example.com/storage\n\ngo 1.27.3\n\nrequire github.com/nsgo-dev/nsgo v0.0.0\n\nreplace github.com/nsgo-dev/nsgo => " + root + "\n",
		"go.sum":    string(sum),
		"nsgo.yaml": "name: Storage\nid: com.example.storage\nweb:\n  storage: indexeddb\n",
		"main.go":   storageTestApp,
	})
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOWORK", "off")
	p, err := load(app)
	if err != nil {
		t.Fatal(err)
	}
	p.Release = true
	if err := buildWeb(t.Context(), p, site, false); err != nil {
		t.Fatal(err)
	}
	files := http.FileServer(http.Dir(site))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The sandboxed page really has an opaque origin, but it can still
		// fetch and instantiate the app so it can report the storage denial.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		switch r.URL.Path {
		case "/embed.html":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, storageTestEmbed)
		case "/sandbox.html":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><script>
addEventListener("message", e => { if (e.source === document.querySelector("iframe").contentWindow) document.body.dataset.result = e.data; });
</script><iframe sandbox="allow-scripts" src="/embed.html?phase=denied"></iframe>`)
		default:
			files.ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	browser, err := startBrowser(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.close()
	page, closePage, err := openWebPage(browser, webPageOptions{
		url: server.URL + "/?phase=write", width: 400, height: 300, out: io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closePage()
	call := func(method string, args any) json.RawMessage {
		t.Helper()
		raw, err := page.call(method, args)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	eval := func(expression string) any {
		t.Helper()
		raw := call("Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true})
		var r struct {
			Result           struct{ Value any }
			ExceptionDetails json.RawMessage
		}
		if err := json.Unmarshal(raw, &r); err != nil || r.ExceptionDetails != nil {
			t.Fatalf("evaluating %s: %s (%v)", expression, raw, err)
		}
		return r.Result.Value
	}
	wait := func(expression, want string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			got, _ := eval(expression).(string)
			if strings.Contains(got, want) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s = %q, want %q", expression, got, want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	navigate := func(path string) { call("Page.navigate", map[string]any{"url": server.URL + path}) }
	const text = "document.body?.innerText"
	wait(text, "storage: written")
	// Seed a different application's real database, using the same keys.
	eval(`(async () => {
	  const db = await new Promise((resolve, reject) => {
	    const r = indexedDB.open("nsgo:com.example.storage.other");
	    r.onupgradeneeded = () => r.result.createObjectStore("entries");
	    r.onsuccess = () => resolve(r.result);
	    r.onerror = () => reject(r.error);
	  });
	  await new Promise((resolve, reject) => {
	    const tx = db.transaction("entries", "readwrite");
	    tx.objectStore("entries").put("other app", "slot");
	    tx.oncomplete = resolve;
	    tx.onabort = () => reject(tx.error);
	  });
	  db.close();
	})()`)
	// Reload in this tab to a fresh instance of the standalone app.
	eval(`history.replaceState(null, "", "/?phase=read")`)
	call("Page.reload", map[string]any{})
	wait(text, "storage: reopened")

	// The embed path also consumes web.storage from the published app.
	// Abort one put after its request succeeds but before the commit.
	navigate("/embed.html?phase=abort")
	wait(text, "storage: aborted and reopened")
	if err := page.key("k"); err != nil {
		t.Fatal(err)
	}
	wait("window.storageKeyResult", "saved from keydown")
	if got := eval(`(async () => {
	  const value = async (id, key) => {
	    const db = await new Promise((resolve, reject) => {
	      const r = indexedDB.open(id);
	      r.onsuccess = () => resolve(r.result);
	      r.onerror = () => reject(r.error);
	    });
	    const v = await new Promise((resolve, reject) => {
	      const r = db.transaction("entries").objectStore("entries").get(key);
	      r.onsuccess = () => resolve(r.result);
	      r.onerror = () => reject(r.error);
	    });
	    db.close();
	    return v;
	  };
	  const app = "nsgo:com.example.storage";
	  return [await value(app, "slot"), await value(app, "key"), await value(app + ".other", "slot")].join("|");
	})()`); got != "old|value|other app" {
		t.Fatalf("committed values = %v", got)
	}

	navigate("/embed.html?phase=quota")
	wait(text, "storage: quota refused")
	if got := eval(`localStorage.getItem("nsgo:com.example.storage:slot")`); got != "old" {
		t.Fatalf("localStorage's previous value = %v", got)
	}

	navigate("/sandbox.html")
	wait("document.body?.dataset.result", "storage: denied and blocked")
}

const storageTestEmbed = `<!doctype html><div id="app" style="height: 200px"></div>
<script type="module">
const phase = new URL(location.href).searchParams.get("phase");
const url = new URL("/nsgo.json", location.href);
const config = await (await fetch(url)).json();
const { mount } = await import(new URL(config.loader, url));
const app = document.querySelector("#app");
if (phase === "quota" || phase === "denied") delete config.storage;
if (phase === "quota") localStorage.setItem("other-app", "x".repeat(4 * 1024 * 1024));
if (phase === "abort") {
  const put = IDBObjectStore.prototype.put;
  IDBObjectStore.prototype.put = function(value, key) {
    const request = put.call(this, value, key);
    if (key === "slot") request.addEventListener("success", () => this.transaction.abort(), { once: true });
    return request;
  };
  new MutationObserver((_, o) => {
    const key = app.querySelector("[data-testid=writer]");
    if (key) { key.focus(); o.disconnect(); }
  }).observe(app, {childList: true, subtree: true});
}
if (phase === "denied") new MutationObserver((_, o) => {
  if (app.innerText.startsWith("storage: ")) { parent.postMessage(app.innerText, "*"); o.disconnect(); }
}).observe(app, {childList: true, subtree: true});
await mount(app, url, {config});
</script>`

const storageTestApp = `package main

import (
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

func main() { nsgo.Run(app) }

func app(cx *core.Scope) core.Node {
	show := func(result string) core.Text { return core.Text{Content: "storage: " + result} }
	g := js.Global()
	s := cx.Storage()
	if err := s.Err(); err != nil {
		if strings.Contains(err.Error(), "SecurityError") && s.SetItem("slot", "unexpected") == err {
			return show("denied and blocked")
		}
		return show(err.Error())
	}
	phase := g.Get("URLSearchParams").New(g.Get("location").Get("search")).Call("get", "phase").String()
	if phase == "quota" {
		if err := s.SetItem("slot", "old"); err != nil { return show(err.Error()) }
		err := s.SetItem("slot", strings.Repeat("z", 2<<20))
		v, _ := s.GetItem("slot")
		if err != nil && strings.Contains(err.Error(), "QuotaExceededError") && v == "old" { return show("quota refused") }
		return show("unexpected quota result")
	}

	const size = 20 << 20
	child := cx.Child()
	child.ProvideStorage("plugin", 100)
	part := child.Storage()
	if phase == "write" {
		if err := s.SetItem("large", "start"+strings.Repeat("x", size-8)+"end"); err != nil { return show(err.Error()) }
		if err := s.SetItem("slot", "old"); err != nil { return show(err.Error()) }
		if err := part.SetItem("slot", "plugin"); err != nil { return show(err.Error()) }
		return show("written")
	}
	v, _ := s.GetItem("large")
	if len(v) != size || !strings.HasPrefix(v, "start") || !strings.HasSuffix(v, "end") { return show("large entry missing") }
	if v, _ := s.GetItem("slot"); v != "old" { return show("app's slot missing") }
	if v, _ := part.GetItem("slot"); v != "plugin" || len(part.Keys()) != 1 || len(s.Keys()) != 2 { return show("partition isolation failed") }
	if phase != "abort" { return show("reopened") }
	err := s.SetItem("slot", "bad")
	v, _ = s.GetItem("slot")
	if err == nil || !strings.Contains(err.Error(), "transaction aborted") || v != "old" { return show("abort did not preserve the previous value") }
	return core.View{
		TestID: "writer", Focusable: func() bool { return true },
		OnKeyDown: func(core.KeyEvent) bool {
			if err := s.SetItem("key", "value"); err != nil { g.Set("storageKeyResult", err.Error()) } else { g.Set("storageKeyResult", "saved from keydown") }
			return true
		},
		Children: []core.Node{show("aborted and reopened")},
	}
}
`
