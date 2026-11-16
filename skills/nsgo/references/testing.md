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

`TestMain` gives the platform the main thread when tests run on a target; keep it in every package with app tests. For setup and teardown around the tests, call `nsgotest.MainFunc(func() int { setup(); defer teardown(); return m.Run() })` instead, never code after `nsgotest.Main`, which does not return; on a target it runs once per test, each in a process of its own. On Android, write setup logs to stderr until the host starts: stdout carries its protocol, and a bypassed host or corrupt frame fails the run with a `nsgotest.Main` hint.

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

- Pointer and touch: `Press`, `PressWith`, `PressAt`, `PressAtWith`, `LongPress`, `RightClick`, `Hover`, `Drag`, `Scroll`, `ScrollTo`, `PullToRefresh`, `Pinch`, `Wheel`.
- Keyboard and text: `Focus`, `Type`, `Clear`, `Submit`, `KeyPress`, `Paste`, `Select`, `Compose`.
- Menus and system UI: `ChooseMenu`, `ChooseDockMenu`, `ChoosePopup`, `ClosePopup`, `HeaderBack`, `Answer` for alerts (start with `nsgotest.PendingAlerts()`), `GrantPermission`, `DropFiles`, `CloseWindow` (with a desktop `Environment`, where the main view shows in the main window as `nsgo.Run` opens one: on macOS, and on Windows and Linux for an app's function).
- Reading: `Find`, `Exists`, `Enabled`, `Query`, `Texts`, `Dump`, and `FindMounted` for views outside the window; a view's `Bounds()` is its frame in the window's points.
- Waiting: every step settles until animations end; on a target it, and `Settle`, first waits for the engine's pending commit, then for 100 ms without batches or finite animations, five seconds at most in total, so never add an empty post to wait for a slow first commit. `WaitFor`, `WaitForGone` and `WaitUntil` wait for what arrives later, such as a `cx.Post` from a goroutine. `Advance` and `FastForward` move the clock, which `cx.Now()` reads, so time what the app measures itself, such as a gesture's speed, with `cx.Now()` rather than `time.Now()`; never `time.Sleep`.
- Options of `Start`: `nsgotest.Size(w, h)` (390 by 844 points by default), `Storage(map)` for saved state, `Secrets(map)` for what the secure store kept, which `app.Secrets` holds in memory, `Environment(core.Environment{...})` for an appearance, a locale or a safe area, and `app.SetEnvironment` to change it later, `Host` for an embedding host, `HotKeyTaken(keys...)` and `HotKeyUnsupported(keys...)` for global shortcuts the headless platform refuses.

`PressWith(selector, core.Modifiers{Shift: true})` and `PressAtWith(selector, at, mods)` hold `Shift`, `Control`, `Alt` or `Platform` for that press only, including its down and up events. Plain presses carry no modifiers. These steps use native input on a target; watchOS rejects modified presses.

## Tests without the network

Keep a test off the network and disk the user owns: inject the fetcher or the `http.Client` into the component, or serve answers from `net/http/httptest`, and give storage and secrets through `nsgotest.Storage` and `nsgotest.Secrets`, so no test reaches the machine's keychain. A resource's loading and error states are then tested by answering slowly or with an error.

## On a platform

Cleanup closes native context menus left open by an app, including between subtests in one process.

```sh
go tool nsgo test -target macos            # windows, linux, web, ios (a connected iPhone), ios_simulator, tvos_simulator, watchos_simulator, android or all
go tool nsgo test -target macos -run TestAdd
```

On macOS, test windows accept only the test host's input, so the developer's mouse, keyboard and trackpad cannot change their scenes while the suite runs.

`app.Screenshot("en/library")` saves `.nsgo/test/screenshots/<target>/en/library.png`, `app.Capture()` returns the pixels, and both skip headless. `-trace` prints each step's commits, `-junit report.xml` writes a report, `-bench` runs benchmarks, `-env KEY=VALUE` reaches a phone's tests, and flags after `--` go to `go test`. `test` takes one directory, every package under it, and errors on a second.

DevTools' Recorder (`go tool nsgo dev -devtools`) writes what you do in the running app as an `nsgotest` test to start from.

With `-target web`, libraries that do not link the backend run headless inside the browser. Storage stays in memory; `DataDir`, `CacheDir` and `LogsDir` are empty, and the file-backed `nsgotest.MicrophoneInput` fake skips.

## TSX

A TSX app's `*.test.tsx` files, written with `nsgo/test`, run with its Go tests under `go tool nsgo test`.
