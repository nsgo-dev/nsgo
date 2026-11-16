---
title: Testing
description: Test an app the way a user uses it, headless or in a native window.
group: Toolchain
order: 21
---

## A test

`nsgotest` starts the app without a window. A test finds controls by their text or role and acts on them as a user would:

```go
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}
```

`TestMain` gives the platform the main thread when the tests run on a target.

An app run with its function, `nsgo.Run(func(app *core.Scope) {...})`, is tested with `nsgotest.StartApp(t, fn)`, which runs the function once in the app's own scope, as `Run` does, so its application menu and quit handlers stay. With a desktop `nsgotest.Environment` the app's main view shows in its main window where `nsgo.Run` opens one: on macOS, and on Windows and Linux for an app's function. There `desktop.CurrentWindow` and `OnClosing` work; `app.CloseWindow()` sends the window's close button's request and reports whether it closed, and a handler alone is tested by calling it and reading `WindowClosing.Canceled()`, or `QuitRequest.Canceled()` for `OnQuitRequest`. Steps act on the main window only. `app.DropFiles` may name anything inside a drop zone: the nearest view around it with `OnFileDrop` takes the files. A view's `Bounds()` is its frame in the window's points, as `Ref.Bounds` reports it. Headless, `app.Press` flips the system switch (`ui.SwitchSystem`) as on a target, and `app.Emit("id:switch", "change", true)` reports a value directly. `app.Navigate` and `app.PostMessage` reach a web view that is mounted but hidden, as it goes on loading.

## Run the tests

```sh
go tool nsgo test
```

`test` prints a line per test. `-run` picks tests as `go test`'s does, including subtests such as `-run 'TestGallery/dark-home'`. On a target, `-timeout` sets the per-test deadline (three minutes by default; `0` disables it) and the result reports when that deadline is reached. `-junit report.xml` writes a JUnit report, `-trace` prints each step's mount items and commit times and `-bench` runs benchmarks instead, a line per result. Flags after `--` go to `go test`.

Given a directory without `nsgo.yaml`, such as a library or the module's root, `test` takes every package under it; a directory may also end in `/...`, as Go's patterns do. `go tool nsgo check` takes packages the same way and vets them.

## Run on a platform

The same tests run in a real window with the platform's own input:

```sh
go tool nsgo test -target macos
```

The targets are `macos`, `windows`, `linux`, `web` (in Chrome, Chromium or Edge), `ios_simulator`, `tvos_simulator` (the remote's arrows, Return as its select button and Escape as Menu), `watchos_simulator` and `android`, or `all`. Each test runs in a process of its own. `app.Capture()` returns the current pixels as an `image.Image` and `app.CapturePNG()` the PNG bytes, both skipping the test headless. Their pixels are in the display's colour space, Display P3 on a Mac with a P3 display: the PNG names it in its colour profile, which `image/png` ignores, so a colour read from `app.Capture()` is that space's and not sRGB's, and NSGo does not convert it. `app.Screenshot("en/library")` saves a screenshot as `.nsgo/test/screenshots/<target>/en/library.png`, `.nsgo/test/screenshots/macos/en/library.png` on `macos`; names may contain spaces but cannot be absolute or contain `..`. `-screenshots dir` saves them under `dir` instead, a path relative to the working directory, and a screenshot whose path an earlier one took fails its test. `-fps` logs the performance monitor's readings each second on `macos`, `windows`, `linux` and `ios_simulator`, and the engine's alone on `web` and `android`. `-env KEY=VALUE` sets a variable of the tests' environment, which a phone's tests see no other way.

To measure rather than check, start with `nsgotest.Measure()`: the app runs on the system's clock, `Advance` waits as long as it moves, and steps wait only for what animates to rest. `app.Mark("name")` names what the readings from there on measure, and `app.Mark("-")` ends it; the marks print with or without `-fps`.

## TSX tests

A TSX app's `*.test.tsx` files, written with `nsgo/test`, run with its Go tests.

To script a TSX app without a test file, pass its entry and the steps on standard input, written as a `*.test.tsx` test's body with `app` the app:

```sh
go tool nsgo test App.tsx < steps.ts
```
