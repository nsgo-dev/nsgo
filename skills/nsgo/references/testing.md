# Testing NSGo apps

`nsgotest` starts an app without a window, on a manual clock, and acts on it as a user does. The same tests run in a real window with the platform's input through `go tool nsgo test -target`.

```go
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestAdd(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Type("id:draft", "Milk")
	app.Press("role:button Add")
	app.WaitFor("text:Milk", time.Second)
}
```

`TestMain` gives the platform the main thread when tests run on a target; keep it in every package with app tests.

An app run with its function, `nsgo.Run(func(app *core.Scope) {...})`, starts with `nsgotest.StartApp(t, fn)`, which runs it once in the app's own scope as `Run` does; never pass `core.AppRoot(fn)` to `Start`, which shows a root view as the main view, so the function's scope would not last.

## Selectors

| Selector | Matches |
| --- | --- |
| `id:<TestID>` | The view whose `TestID` field is that |
| `label:<Label>` | The view with that accessibility label |
| `text:<text>` | A text containing it, or a field holding it (its placeholder while empty) |
| `role:<role>` | A view with that role: `button`, `checkbox`, `textbox`, ... |
| `role:<role> <name>` | One whose label is name, or whose texts contain it |
| anything else | A test ID, else a label, else a whole text |

`label:`, `text:` and a role's name take `/regexp/flags`. `a >> b` looks for `b` inside `a`, and `>> nth=<i>` keeps one match. A step fails the test when its selector matches no view, more than one, or a disabled one, and prints the view tree; that is the check, so a separate assertion is rarely needed.

Prefer what a user sees, text and roles, and set `TestID` on elements where text repeats or is translated. Tests read words in the language they run in.

## Steps

- Pointer and touch: `Press`, `LongPress`, `RightClick`, `Hover`, `Drag`, `Scroll`, `ScrollTo`, `PullToRefresh`, `Pinch`, `Wheel`.
- Keyboard and text: `Focus`, `Type`, `Clear`, `Submit`, `KeyPress`, `Paste`, `Select`, `Compose`.
- Menus and system UI: `ChooseMenu`, `ChoosePopup`, `ClosePopup`, `HeaderBack`, `Answer` for alerts (start with `nsgotest.PendingAlerts()`), `GrantPermission`, `DropFiles`, `CloseWindow` (with a desktop `Environment`, where the main view shows in the main window as `nsgo.Run` opens one: on macOS, and on Windows and Linux for an app's function).
- Reading: `Find`, `Exists`, `Enabled`, `Query`, `Texts`, `Dump`, and `FindMounted` for views outside the window; a view's `Bounds()` is its frame in the window's points.
- Waiting: every step settles until animations end; `WaitFor`, `WaitForGone` and `WaitUntil` wait for what arrives later, such as a `cx.Post` from a goroutine. `Advance` and `FastForward` move the clock; never `time.Sleep`.
- Options of `Start`: `nsgotest.Size(w, h)` (390 by 844 points by default), `Storage(map)` for saved state, `Environment(core.Environment{...})` for an appearance, a locale or a safe area, and `app.SetEnvironment` to change it later, `Host` for an embedding host, `HotKeyTaken(keys...)` and `HotKeyUnsupported(keys...)` for global shortcuts the headless platform refuses.

## Tests without the network

Keep a test off the network and disk the user owns: inject the fetcher or the `http.Client` into the component, or serve answers from `net/http/httptest`, and give storage through `nsgotest.Storage`. A resource's loading and error states are then tested by answering slowly or with an error.

## On a platform

```sh
go tool nsgo test -target macos            # windows, linux, web, ios_simulator, tvos_simulator, watchos_simulator, android or all
go tool nsgo test -target macos -run TestAdd
```

`app.Screenshot("en/library")` saves `.nsgo/test/screenshots/<target>/en/library.png`, `app.Capture()` returns the pixels, and both skip headless. `-trace` prints each step's commits, `-junit report.xml` writes a report, `-bench` runs benchmarks, `-env KEY=VALUE` reaches a phone's tests, and flags after `--` go to `go test`. `test` takes one directory, every package under it, and errors on a second.

DevTools' Recorder (`go tool nsgo dev -devtools`) writes what you do in the running app as an `nsgotest` test to start from.

## TSX

A TSX app's `*.test.tsx` files, written with `nsgo/test`, run with its Go tests under `go tool nsgo test`.
