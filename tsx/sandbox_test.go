package tsx_test

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	baselocales "github.com/nsgo-dev/nsgo/base/locales"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx"
	"github.com/nsgo-dev/nsgo/tsx/bundler"
	"github.com/nsgo-dev/nsgo/tsx/pkg"

	// NSGo's packages that register into its modules, for the sandbox's surface.
	_ "github.com/nsgo-dev/nsgo/tsx/diff"
	_ "github.com/nsgo-dev/nsgo/tsx/editor"
	_ "github.com/nsgo-dev/nsgo/tsx/terminal"
	_ "github.com/nsgo-dev/nsgo/tsx/ui"
	_ "github.com/nsgo-dev/nsgo/tsx/ui/chat"
	_ "github.com/nsgo-dev/nsgo/tsx/ui/richeditor"
)

// verified compiles source and signs it with caps as com.example.plugin,
// returning it verified.
func verified(t *testing.T, source string, caps pkg.Capabilities) *pkg.Package {
	t.Helper()
	code, err := bundler.Compile(fstest.MapFS{"App.tsx": {Data: []byte(source)}}, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := pkg.GenerateKey()
	release, _ := pkg.GenerateKey()
	cert, err := pkg.Certify(root, release.Public().(ed25519.PublicKey), []string{"com.example.*"}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	data, err := pkg.Build(pkg.Contents{App: pkg.App{ID: "com.example.plugin", Build: 1}, Source: code, Capabilities: caps}, release, cert)
	if err != nil {
		t.Fatal(err)
	}
	trust := pkg.NewTrust()
	trust.AddRoot(root.Public().(ed25519.PublicKey))
	p, err := trust.Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// network answers every request with its URL, and redirects /away to
// evil.io.
func network(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/away" {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://evil.io/"}},
			Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	if req.URL.Path == "/big" {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 100))), Request: req}, nil
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(req.URL.String())), Request: req}, nil
}

// tries is a plugin that runs each named attempt and shows what each
// gave, in order, as "name: result".
const tries = `import { mount, signal, Text, AuthSession, Clipboard, openUrl, Storage, host } from "nsgo";
const log = signal([]);
const say = (name, v) => log.set([...log(), name + ": " + v]);
const how = (e) => e.code ? e.code + " " + (e.permission ?? "") : e.name + " " + e.message;
const attempt = async (name, fn) => { try { say(name, await fn()); } catch (e) { say(name, how(e)); } };
(async () => {
	for (const [name, fn] of Object.entries(attempts)) await attempt(name, fn);
	say("done", 1);
})();
mount(() => <Text testID="log">{() => log().join("\n")}</Text>);
`

// results runs the attempts in a View of caps and returns what each gave.
func results(t *testing.T, attempts string, caps pkg.Capabilities, s tsx.Sandbox) map[string]string {
	t.Helper()
	p := verified(t, "const attempts = {"+attempts+"};\n"+tries, caps)
	// What Go holds the package to, which holds on the web too.
	s.WebUnisolated = true
	s.Limits = &tsx.Limits{Load: 10 * time.Second, Call: 2 * time.Second, Body: 50, Requests: 2, Timers: 3, Storage: 64}
	app := nsgotest.Start(t, tsx.WithFetcher(func(cx *core.Scope) core.Node {
		return tsx.View{Package: p, Sandbox: s}
	}, network))
	app.WaitUntil(10*time.Second, func() bool { return strings.Contains(app.Find("id:log").Text, "done: 1") })
	out := map[string]string{}
	for _, line := range strings.Split(app.Find("id:log").Text, "\n") {
		k, v, _ := strings.Cut(line, ": ")
		out[k] = v
	}
	return out
}

func TestSandboxHoldsAPackageToItsCapabilities(t *testing.T) {
	got := results(t, `
	allowed: async () => (await fetch("https://api.example.com/a")).text(),
	other: () => fetch("https://evil.io/"),
	redirect: () => fetch("https://api.example.com/away"),
	file: () => fetch("file:///etc/hosts"),
	big: async () => (await fetch("https://api.example.com/big")).text(),
	data: async () => (await fetch("data:,hi")).text(),
	clipboard: () => Clipboard.readText(),
	openUrl: () => openUrl("https://example.com"),
	auth: () => AuthSession.start((redirect) => "https://provider.example/?redirect_uri=" + redirect).result,
	granted: () => host.call("greet", "Ada"),
	refused: () => host.call("secret"),
	timers: () => { for (let i = 0; i < 5; i++) setTimeout(() => {}, 100000); },
	storage: async () => { await Storage.setItem("k", "v"); return (await Storage.keys()).join(","); },
	full: () => Storage.setItem("big", "x".repeat(100)),
	`, pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"api.example.com"}}, Host: []string{"greet"}}, tsx.Sandbox{Host: greeter()})
	want := map[string]string{
		"allowed":   "https://api.example.com/a",
		"other":     "ERR_ACCESS_DENIED network",
		"redirect":  "ERR_ACCESS_DENIED network",
		"file":      "ERR_ACCESS_DENIED network",
		"big":       "ERR_NSH_QUOTA_EXCEEDED ",
		"data":      "hi",
		"clipboard": "ERR_ACCESS_DENIED clipboard",
		"openUrl":   "ERR_ACCESS_DENIED openUrl",
		"auth":      "ERR_ACCESS_DENIED authSession",
		"granted":   "hi Ada",
		"refused":   "ERR_ACCESS_DENIED host",
		"timers":    "ERR_NSH_QUOTA_EXCEEDED ",
		"storage":   "k",
		"full":      "ERR_NSH_QUOTA_EXCEEDED ",
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s gave %q, want %q", k, got[k], w)
		}
	}
}

func TestSandboxHoldsWhatWebPagesCapture(t *testing.T) {
	p := verified(t, `import { mount, View, WebView } from "nsgo";
mount(() => <View>
	<WebView testID="web" src="https://call.example.com/" media={["microphone", "camera"]} style={{ height: 50 }} />
</View>);`, pkg.Capabilities{WebView: &pkg.WebView{Hosts: []string{"call.example.com"}, Media: []string{"microphone"}}})
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node { return tsx.View{Package: p, Sandbox: tsx.Sandbox{WebUnisolated: true}} })
	app.WaitFor("id:web", 10*time.Second)
	if v := app.Find("id:web"); v.Web.Source.Src != "https://call.example.com/" || v.Web.Media != core.WebMicrophone {
		t.Fatalf("the web view loads %q capturing %v", v.Web.Source.Src, v.Web.Media)
	}
}

// TestTheWebRefusesWhatItCannotIsolate runs a package with capabilities: the page's engine cannot isolate it, so on the web View and Load refuse it unless the host opts in.
func TestTheWebRefusesWhatItCannotIsolate(t *testing.T) {
	p := verified(t, `import { mount, Text } from "nsgo";
mount(() => <Text testID="ran">ran</Text>);`, pkg.Capabilities{Clipboard: true})
	var viewErr, loadErr error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		_, loadErr = tsx.Load(cx, p, tsx.Sandbox{})
		return core.View{Children: []core.Node{
			core.Catch{Try: tsx.View{Package: p}.Render, Fallback: func(cx *core.Scope, err error, _ func()) core.Node {
				viewErr = err
				return core.Text{Content: "refused", TestID: "refused"}
			}},
			tsx.View{Package: p, Sandbox: tsx.Sandbox{WebUnisolated: true}},
		}}
	})
	if runtime.GOOS == "js" {
		if !errors.Is(viewErr, tsx.ErrUnisolated) || !errors.Is(loadErr, tsx.ErrUnisolated) || app.Find("id:refused").Text != "refused" {
			t.Fatalf("the view failed with %v and the plugin with %v\n%s", viewErr, loadErr, app.Dump())
		}
		if !strings.Contains(viewErr.Error(), "cannot isolate") {
			t.Fatalf("the refusal says %q", viewErr)
		}
	} else if viewErr != nil || loadErr != nil {
		t.Fatalf("the view failed with %v and the plugin with %v", viewErr, loadErr)
	}
	if !app.Exists("id:ran") {
		t.Fatalf("the package the host opted in for did not run\n%s", app.Dump())
	}
}

func greeter() *core.Host {
	h := core.NewHost()
	h.Handle("greet", func(c *core.HostCall) { c.Resolve("hi " + c.Data.(string)) })
	h.Handle("secret", func(c *core.HostCall) { c.Resolve("leaked") })
	return h
}

func TestGrantNarrowsCapabilities(t *testing.T) {
	got := results(t, `
	a: async () => (await fetch("https://a.com/")).text(),
	b: () => fetch("https://b.com/"),
	`, pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"a.com", "b.com"}}},
		tsx.Sandbox{Grant: &pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"a.com"}}}})
	if got["a"] != "https://a.com/" || got["b"] != "ERR_ACCESS_DENIED network" {
		t.Fatalf("got %v", got)
	}
}

type sandboxClass struct{}

func init() {
	tsx.Register("sandboxtest", map[string]core.Node{"Marker": core.Text{}}, tsx.SandboxSafe())
	tsx.RegisterFunctions("nsgo/base", map[string]any{"sandboxSecret": func() string { return "secret" }})
	// What NSGo's modules hold without SandboxSafe: a component, a class and a static of a component sandboxes import.
	tsx.Register("nsgo/base", map[string]core.Node{"SandboxHidden": core.Text{}})
	tsx.RegisterClass("nsgo/base", "SandboxHiddenClass", tsx.Class[sandboxClass]{New: func() *sandboxClass { return &sandboxClass{} }})
	tsx.RegisterStatics("nsgo/base", "Button", map[string]any{"sandboxHidden": func() string { return "secret" }})
}

func TestSandboxImportsOnlyWhatItIsGiven(t *testing.T) {
	attempts := `
	base: async () => typeof (await import("nsgo/base")).Button,
	fn: async () => typeof (await import("nsgo/base")).sandboxSecret,
	component: async () => typeof (await import("nsgo/base")).SandboxHidden,
	class: async () => typeof (await import("nsgo/base")).SandboxHiddenClass,
	static: async () => typeof (await import("nsgo/base")).Button.sandboxHidden,
	theme: async () => typeof (await import("nsgo/base")).Scrollbar.setTheme,
	scheme: async () => typeof (await import("nsgo")).setColorScheme,
	exit: async () => typeof (await import("nsgo/mobile")).BackHandler.exitApp,
	app: async () => typeof (await import("sandboxtest")).Marker,
	`
	got := results(t, attempts, pkg.Capabilities{}, tsx.Sandbox{})
	for _, k := range []string{"fn", "component", "class", "static", "theme", "scheme", "exit"} {
		if got[k] != "undefined" {
			t.Errorf("by default, %s is %s", k, got[k])
		}
	}
	if got["base"] != "function" || !strings.Contains(got["app"], "not given to this sandbox") {
		t.Fatalf("by default: %v", got)
	}
	got = results(t, attempts, pkg.Capabilities{}, tsx.Sandbox{Modules: []string{"nsgo/base.sandboxSecret", "nsgo/base.SandboxHidden",
		"nsgo/base.SandboxHiddenClass", "nsgo/base.Button.sandboxHidden", "nsgo.setColorScheme", "nsgo/mobile.BackHandler.exitApp", "sandboxtest"}})
	for _, k := range []string{"fn", "component", "class", "static", "scheme", "exit", "app"} {
		if got[k] != "function" {
			t.Errorf("given, %s is %s", k, got[k])
		}
	}
	if got["theme"] != "undefined" {
		t.Errorf("given others, setTheme is %s", got["theme"])
	}
}

// outsideNSGo is what registering into nsgo/base panicked with when the caller was not NSGo's: reflect's call, before any app started.
var outsideNSGo = func() (msg string) {
	defer func() { msg = fmt.Sprint(recover()) }()
	reflect.ValueOf(tsx.Register).Call([]reflect.Value{reflect.ValueOf("nsgo/base"),
		reflect.ValueOf(map[string]core.Node{"Outsider": core.Text{}}), reflect.ValueOf(tsx.SandboxSafe())})
	return ""
}()

func TestRegistryIsNSGosAndSetBeforeApps(t *testing.T) {
	if !strings.Contains(outsideNSGo, "the nsgo modules are NSGo's own") {
		t.Errorf("a caller outside NSGo registered into nsgo/base: %q", outsideNSGo)
	}
	nsgotest.Start(t, func(cx *core.Scope) core.Node { return tsx.App("late.js", `require("nsgo");`)(cx) })
	for name, register := range map[string]func(){
		"Register":          func() { tsx.Register("app/late", map[string]core.Node{"Late": core.Text{}}) },
		"RegisterClass":     func() { tsx.RegisterClass("app/late", "Late", tsx.Class[sandboxClass]{}) },
		"RegisterStatics":   func() { tsx.RegisterStatics("app/late", "Late", map[string]any{"f": func() {}}) },
		"RegisterFunctions": func() { tsx.RegisterFunctions("app/late", map[string]any{"f": func() {}}) },
		"RegisterMessages":  func() { tsx.RegisterMessages("app/late", baselocales.In, baselocales.Parameters) },
	} {
		func() {
			defer func() {
				if msg := fmt.Sprint(recover()); !strings.Contains(msg, "after an app started") {
					t.Errorf("%s after an app started: %q", name, msg)
				}
			}()
			register()
		}()
	}
}

// What a default Sandbox imports, every export of SandboxModules and the members of those that are functions (statics) or objects (services), with all of NSGo's packages that register into them linked, is testdata/sandbox-surface.txt, so whatever widens it is a reviewed change; NSGO_UPDATE_SANDBOX_SURFACE writes it anew.
func TestSandboxSurface(t *testing.T) {
	modules, _ := json.Marshal(tsx.SandboxModules)
	p := verified(t, `const modules = `+string(modules)+`;
export function surface() {
	const out = [];
	for (const name of modules) {
		const m = require(name);
		for (const k of Object.keys(m).sort()) {
			out.push(name + "." + k);
			const v = m[k];
			if (typeof v === "function" || typeof v === "object" && v !== null) for (const s of Object.keys(v).sort()) out.push(name + "." + k + "." + s);
		}
	}
	return out.join("\n") + "\n";
}`, pkg.Capabilities{})
	var plugin *tsx.Plugin
	var loadErr error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		plugin, loadErr = tsx.Load(cx, p, tsx.Sandbox{})
		return core.View{}
	})
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	var got any
	var callErr error
	app.Post(func() { plugin.Call("surface", nil, func(v any, err error) { got, callErr = v, err }) })
	app.WaitUntil(5*time.Second, func() bool { return got != nil || callErr != nil })
	if callErr != nil {
		t.Fatal(callErr)
	}
	const name = "testdata/sandbox-surface.txt"
	if os.Getenv("NSGO_UPDATE_SANDBOX_SURFACE") != "" {
		if err := os.WriteFile(name, []byte(got.(string)), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := testdata.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("a default Sandbox imports other than %s; review the change and write it anew with NSGO_UPDATE_SANDBOX_SURFACE=1:\n%s", name, got)
	}
}

// A plugin's key bindings, when: "always" included, and back handlers fire only while the focus is in what it shows: the host's field keeps the host's shortcut and back.
func TestSandboxKeysAndBackStayInItsView(t *testing.T) {
	p := verified(t, `import { mount, signal, keymap, View, Text } from "nsgo";
import { Input } from "nsgo/base";
import { BackHandler } from "nsgo/mobile";
const log = signal("");
keymap.bind("mod+s", () => log.set(log() + "save;"), { when: "always" });
BackHandler.onBackPress(() => { log.set(log() + "back;"); return true; });
mount(() => <View><Input testID="plugin-field" /><Text testID="plugin-log">{() => "plugin " + log()}</Text></View>);`, pkg.Capabilities{})
	var host []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.OnKey("mod+s", func() { host = append(host, "save") })
		cx.OnBackPress(func() bool { host = append(host, "back"); return true })
		return core.View{Children: []core.Node{core.Input{TestID: "host-field"}, tsx.View{Package: p}}}
	})
	app.Focus("id:host-field")
	app.Key("mod+s")
	app.Back()
	if got := app.Find("id:plugin-log").Text; got != "plugin " || !slices.Equal(host, []string{"save", "back"}) {
		t.Fatalf("focused in the host's field: the plugin's log %q, the host's %q", got, host)
	}
	app.Focus("id:plugin-field")
	app.Key("mod+s")
	app.Back()
	if got := app.Find("id:plugin-log").Text; got != "plugin save;back;" || len(host) != 2 {
		t.Fatalf("focused in the plugin's field: the plugin's log %q, the host's %q", got, host)
	}
}

// A plugin takes the focus on its own only while it has it or as the user's press on it opens what asks for it, so a timer cannot move the focus, and the host's keys with it, from the host's field; an app not sandboxed moves it as it likes.
func TestSandboxTakesTheFocusOnlyFromTheUser(t *testing.T) {
	const source = `import { mount, signal, keymap, View, Text } from "nsgo";
import { Input, Dialog } from "nsgo/base";
const log = signal(""), open = signal(false);
keymap.bind("mod+s", () => log.set(log() + "save;"), { when: "always" });
let field, other;
setTimeout(() => field.focus(), 100);
setTimeout(() => other.focus(), 300);
mount(() => <View>
	<Input testID="plugin-field" ref={field} />
	<Input testID="plugin-other" ref={other} />
	<Text testID="open" onPress={() => open.set(true)}>Open</Text>
	<Dialog open={open()}><Input testID="dialog-field" autoFocus /></Dialog>
	<Text testID="plugin-log">{() => "plugin " + log()}</Text>
</View>);`
	for _, sandboxed := range []bool{true, false} {
		name := map[bool]string{true: "sandboxed", false: "not sandboxed"}[sandboxed]
		var plugin func(cx *core.Scope) core.Node
		if sandboxed {
			p := verified(t, source, pkg.Capabilities{})
			plugin = tsx.View{Package: p}.Render
		} else {
			code, err := bundler.Compile(fstest.MapFS{"App.tsx": {Data: []byte(source)}}, "App.tsx")
			if err != nil {
				t.Fatal(err)
			}
			plugin = tsx.App("App.js", string(code))
		}
		var host []string
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			cx.OnKey("mod+s", func() { host = append(host, "save") })
			return core.View{Children: []core.Node{core.Input{TestID: "host-field"}, core.Dynamic(plugin)}}
		})
		focused := func(what, want string) {
			t.Helper()
			for _, id := range []string{"host-field", "plugin-field", "plugin-other", "dialog-field"} {
				if app.Exists("id:"+id) && app.Find("id:"+id).Focused != (id == want) {
					t.Errorf("%s, %s: %s focused %v", name, what, id, !(id == want))
				}
			}
		}
		app.Focus("id:host-field")
		app.Advance(150 * time.Millisecond)
		if sandboxed {
			focused("after a timer's focus", "host-field")
			app.Key("mod+s")
			if got := app.Find("id:plugin-log").Text; got != "plugin " || !slices.Equal(host, []string{"save"}) {
				t.Errorf("after a timer's focus: the plugin's log %q, the host's %q", got, host)
			}
		} else {
			focused("after a timer's focus", "plugin-field")
		}
		app.Focus("id:plugin-field")
		app.Advance(200 * time.Millisecond)
		focused("after a timer's focus while it has the focus", "plugin-other")
		app.Focus("id:host-field")
		app.Press("id:open")
		app.WaitFor("id:dialog-field", time.Second)
		focused("after a press on it opened a dialog", "dialog-field")
	}
}

// A plugin shows in a view of its own that clips it: its views and overlays stay in it, sized to it, and a dialog it opens on a timer is refused while one its press opens shows; an app not sandboxed shows them over the host's views.
func TestSandboxShowsOnlyWithinItsView(t *testing.T) {
	const source = `import { mount, signal, View, Text, Overlay } from "nsgo";
import { Dialog } from "nsgo/base";
const open = signal(false);
setTimeout(() => open.set(true), 100);
mount(() => <View testID="plugin-root">
	<View testID="plugin-cover" style={{ position: "absolute", top: -40, left: 0, width: 100, height: 100 }} />
	<Overlay testID="plugin-overlay" pointerEvents="none" style={{ width: "100%", height: "100%" }} />
	<Text testID="close" onPress={() => open.set(false)}>Close</Text>
	<Text testID="open" onPress={() => open.set(true)}>Open</Text>
	<Dialog open={open()}><Text testID="dialog-text">Hi</Text></Dialog>
</View>);`
	for _, sandboxed := range []bool{true, false} {
		name := map[bool]string{true: "sandboxed", false: "not sandboxed"}[sandboxed]
		var plugin func(cx *core.Scope) core.Node
		if sandboxed {
			plugin = tsx.View{Package: verified(t, source, pkg.Capabilities{})}.Render
		} else {
			code, err := bundler.Compile(fstest.MapFS{"App.tsx": {Data: []byte(source)}}, "App.tsx")
			if err != nil {
				t.Fatal(err)
			}
			plugin = tsx.App("App.js", string(code))
		}
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Width: core.Pt(100), Height: core.Pt(200)}, Children: []core.Node{
				core.View{TestID: "host-button", OnPress: func(core.PressEvent) {}, Style: core.Style{Height: core.Pt(40)}},
				core.View{TestID: "host-slot", Style: core.Style{Height: core.Pt(60)}, Children: []core.Node{core.Dynamic(plugin)}},
			}}
		})
		frame, overlay := app.Find("id:plugin-root").Parent, app.Find("id:plugin-overlay")
		if sandboxed {
			if frame == app.Find("id:host-slot") || !frame.Style.Clip || frame.Frame.Size != (core.Size{Width: 100, Height: 60}) {
				t.Errorf("%s: the plugin's view is the host's %v, clips %v, at %+v", name, frame == app.Find("id:host-slot"), frame.Style.Clip, frame.Frame.Size)
			}
			if overlay.Parent != frame || overlay.Frame.Size != frame.Frame.Size {
				t.Errorf("%s: the overlay is in its view %v at %+v", name, overlay.Parent == frame, overlay.Frame.Size)
			}
		} else if overlay.Frame.Size.Height <= 60 || app.Find("id:host-slot").Style.Clip {
			t.Errorf("%s: the overlay is %+v and the host's slot clips %v", name, overlay.Frame.Size, app.Find("id:host-slot").Style.Clip)
		}
		app.Advance(150 * time.Millisecond)
		if shown := app.Exists("id:dialog-text"); shown == sandboxed {
			t.Errorf("%s: a dialog a timer opened shows %v", name, shown)
		}
		app.Press("id:close")
		app.Press("id:open")
		if !app.Exists("id:dialog-text") {
			t.Errorf("%s: a dialog its press opened does not show", name)
		}
	}
}

// A plugin taller than the view its host gives it is held to that view, so it cannot grow over the host's views below; an app not sandboxed overflows it.
func TestSandboxStaysInTheRoomItIsGiven(t *testing.T) {
	const source = `import { mount, View } from "nsgo";
mount(() => <View testID="plugin-root" onPress={() => {}} style={{ height: 300 }} />);`
	for _, sandboxed := range []bool{true, false} {
		name := map[bool]string{true: "sandboxed", false: "not sandboxed"}[sandboxed]
		var plugin func(cx *core.Scope) core.Node
		if sandboxed {
			plugin = tsx.View{Package: verified(t, source, pkg.Capabilities{})}.Render
		} else {
			code, err := bundler.Compile(fstest.MapFS{"App.tsx": {Data: []byte(source)}}, "App.tsx")
			if err != nil {
				t.Fatal(err)
			}
			plugin = tsx.App("App.js", string(code))
		}
		pressed := false
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Width: core.Pt(100), Height: core.Pt(200)}, Children: []core.Node{
				// Below the plugin's view, and under it in the order views stack in.
				core.View{TestID: "host-button", OnPress: func(core.PressEvent) { pressed = true },
					Style: core.Style{Position: core.Absolute, Top: core.Pt(60), Width: core.Pt(100), Height: core.Pt(40)}},
				core.View{TestID: "host-slot", Style: core.Style{Height: core.Pt(60)}, Children: []core.Node{core.Dynamic(plugin)}},
			}}
		})
		frame := app.Find("id:plugin-root").Parent
		app.PressAt("id:host-button", driver.Point{X: 50, Y: 20})
		if sandboxed && (frame.Frame.Size.Height != 60 || !pressed) {
			t.Errorf("%s: the plugin's view is %v tall and a press on the host's button reached it %v", name, frame.Frame.Size.Height, pressed)
		}
		if !sandboxed && pressed {
			t.Errorf("%s: a press on the host's button reached it, not the app's view over it", name)
		}
	}
}

func TestStopsAtTheNodeQuota(t *testing.T) {
	p := verified(t, `import { mount, signal, View, Text } from "nsgo";
const n = signal(1);
mount(() => <View>
	<Text testID="more" onPress={() => n.set(100)}>more</Text>
	{() => Array.from({ length: n() }, () => <View />)}
</View>);`, pkg.Capabilities{})
	var failed error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.Catch{Try: tsx.View{Package: p, Sandbox: tsx.Sandbox{Limits: &tsx.Limits{Nodes: 50}, WebUnisolated: true}}.Render,
			Fallback: func(cx *core.Scope, err error, _ func()) core.Node {
				failed = err
				return core.Text{Content: "stopped", TestID: "stopped"}
			}}
	})
	app.Press("id:more")
	app.WaitFor("id:stopped", 5*time.Second)
	if core.ErrorCode(failed) != core.CodeQuotaExceeded {
		t.Fatalf("stopped with %v", failed)
	}
}

func TestLoadCallsWhatAPluginExports(t *testing.T) {
	p := verified(t, `export function add(a, b) { return a + b; }
export async function search(q) { const r = await fetch("https://api.example.com/" + q.term); return { url: await r.text() }; }
export async function fails() { throw new Error("no luck"); }
export const notAFunction = 1;`, pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"api.example.com"}}})
	var plugin *tsx.Plugin
	var loadErr error
	app := nsgotest.Start(t, tsx.WithFetcher(func(cx *core.Scope) core.Node {
		plugin, loadErr = tsx.Load(cx, p, tsx.Sandbox{WebUnisolated: true})
		return core.View{}
	}, network))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	exports := plugin.Exports()
	slices.Sort(exports)
	if !slices.Equal(exports, []string{"add", "fails", "search"}) {
		t.Fatalf("exports %v", exports)
	}
	got := map[string]any{}
	call := func(name string, args ...any) {
		app.Post(func() {
			plugin.Call(name, args, func(v any, err error) {
				if err != nil {
					got[name] = err
				} else {
					got[name] = v
				}
			})
		})
	}
	call("add", 1, 2)
	call("search", map[string]any{"term": "cats"})
	call("fails")
	call("missing")
	app.WaitUntil(5*time.Second, func() bool { return len(got) == 4 })
	if got["add"] != int64(3) && got["add"] != float64(3) {
		t.Errorf("add gave %#v", got["add"])
	}
	if m, _ := got["search"].(map[string]any); m["url"] != "https://api.example.com/cats" {
		t.Errorf("search gave %#v", got["search"])
	}
	if err, _ := got["fails"].(error); err == nil || err.Error() != "no luck" {
		t.Errorf("fails gave %v", got["fails"])
	}
	if err, _ := got["missing"].(error); !errors.Is(err, tsx.ErrNoExport) {
		t.Errorf("missing gave %v", got["missing"])
	}
	if runtime.GOOS == "js" {
		return
	}
	app.Post(plugin.Close)
	var closed error
	app.Post(func() { plugin.Call("add", []any{1, 2}, func(_ any, err error) { closed = err }) })
	app.Settle()
	if closed == nil {
		t.Fatal("a closed plugin was called")
	}
}
