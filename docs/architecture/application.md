# Application and windows

An app opens what it shows itself, as an Avalonia application does: windows, panels, tray icons and global shortcuts, on one engine whose windows share its signals. Read this before changing `nsgo.Run`, `nsgo.SetMainView`, `desktop`'s windows, tray icons, shortcuts or shutdown, or how the engine attaches surfaces. macOS opens the full set, Windows and Linux open them for `nsgo.Run(App)`, and Linux also handles global shortcuts; the [roadmap](../roadmap.md#application-and-windows) orders the remaining Windows and Linux work.

## Model

`nsgo.Run` takes a root view or the app's function. The function runs once, in the app's own scope, and decides what to open:

```go
func main() { nsgo.Run(App, nsgo.HideDock()) }

func App(app *core.Scope) {
	desktop.SetShutdownMode(app, desktop.ShutdownOnExplicit)
	panel := desktop.OpenPanel(app, desktop.PanelOptions{HideOnBlur: true}, Panel)
	desktop.NewTray(app, desktop.TrayOptions{View: TrayIcon, Panel: panel, Menu: trayMenu})
	launcher := desktop.OpenPanel(app, desktop.PanelOptions{Width: 480, Height: 64, HideOnBlur: true}, Launcher)
	desktop.RegisterHotKey(app, "ctrl+alt+space", launcher.Toggle)
}
```

| App | Opens |
| --- | --- |
| Desktop app | `nsgo.Run(Root)`: a main window, the app ending as it closes; others, such as settings, with `OpenWindow` |
| Tray app | A tray icon whose primary click shows a panel and secondary click a menu; an optional welcome window; `ShutdownOnExplicit` and `nsgo.HideDock` |
| Background app | Global shortcuts that show a panel, as Raycast's; an optional welcome window; `ShutdownOnExplicit` |
| Phone or web app | The view `nsgo.SetMainView` sets, or the root view; `desktop`'s functions do nothing |

`examples/tray` is a tray app with all of them.

## Surfaces

The engine runs one graph with a tree per surface (`core.Tree`). Where the platform opens windows (`core.RootOptions.Windows`, the main window's options), the first tree is the app's own surface: it shows no view, and the platform gives it no window, or a hidden one on Windows.

- `OpenWindow`, `OpenPanel` and a drawn tray icon make a tree under a scope of the opener's owner, so the window closes when that owner goes. Every scope under it knows its tree (`Scope.surf`), so services, key bindings and `desktop`'s window functions act on the window the code is in, and the app's own scope on the first tree.
- Each tree has its own `Env`: size, scale, safe area and title bar. What the app forces, its locale, appearance and host tokens, is one signal shared by every window's.
- On desktops, `Env.Displays` reports each display's whole and visible frame, scale and primary flag in the same coordinates as `desktop.SetWindowFrame`, so a saved frame can be placed on a chosen display.
- A panel takes the keyboard without activating the app where the platform can. With `PanelOptions.NoActivate` it never takes it, shown, focused or clicked, so the app the user types in keeps its focus and caret, as an input method's candidate window needs. `Window.ShowAt` shows a panel beside a rectangle in `Env.Displays`' coordinates, such as a caret's or the pointer's, on a `core.Placement` side, flipped to the opposite side when only that one fits and clamped into the visible frame of the display that holds it (`driver.PlaceWindow`); `Window.MoveTo` moves a panel that shows there without showing or focusing it, and both keep the placement for later shows until a tray icon shows the panel under itself. A platform that cannot place it leaves it hidden and reports `EventWindowPlacementFailed`, which `Window.OnPlacementError` hears.
- Opening sends `ItemOpenSurface` to the app's surface. The platform makes the window and attaches it: `engine.Engine.Attach(id, platform)` returns the surface's `Engine`, which reports its size, environment and events as the first surface's does.
- The engine commits every attached tree after each wake-up, the first first, and asks a window's platform for display frames, since the app's surface has no display. A tree waits uncommitted until it is attached.
- A closed window's tree ends: the engine sends its platform one batch, `WindowDestroy`, and forgets it, or sends it to a platform that attaches after the close.
- Where the platform opens no windows (phones, the web, `nsgo.NewView`, the test hosts of targets, `nsgotest` headless except where a desktop environment has it open windows as `Run` would ([testing](testing.md)), and Windows and Linux apps run with a root view), the first tree is the one surface: `OpenWindow`, `OpenPanel` and, except on Windows and Linux, `NewTray` return nil, whose methods do nothing, and `core.AppRoot` shows the main view in it; Android opens one surface beside it, `mobile.OpenOverlay`'s window over other apps ([Android floating windows](android-floating-windows.md)). A platform that registers global shortcuts there, as Linux's own window does and `nsgotest` does, says so with `core.RootOptions.HotKeys`, and the shortcuts go to that surface.
- `ItemHotKeys` sends the app's whole set of global shortcuts whenever it changes, and the platform answers each shortcut's ID with `EventHotKeyState`: registered, taken by another registration, or unsupported. `desktop.RegisterHotKey`'s handle holds the answer in a signal, pending until it comes and again while a changed set registers anew; where no surface takes global shortcuts it is unsupported at once. `nsgotest` registers every shortcut but a second of one combination, and answers the combinations `nsgotest.HotKeyTaken` and `HotKeyUnsupported` name as they say.

## Lifecycle

- `SetShutdownMode`: `ShutdownOnLastWindowClose` (the default), `ShutdownOnMainWindowClose` or `ShutdownOnExplicit`, which only `Scope.Quit` ends. Panels and tray views are not windows here.
- `OnQuit` handlers clean up as the app ends, however it does, with 200 ms for their work ([services](services.md#appearance-and-quitting)).
- `desktop.OnQuitRequest` hears a quit the user asks for on macOS, such as Cmd+Q or a tray menu's Quit (`EventQuitRequest`). Handlers run newest first and can cancel with `q.Cancel()` or defer with `done := q.Defer()`, then answer with `done(true)` or `done(false)`; a second request waits for that answer, and `Scope.Quit` and `Restart` remain immediate ([services](services.md#appearance-and-quitting)).
- A close the user asks for is `EventWindowClose`: the window's `OnClosing` handlers run newest first and may cancel it, as a window that hides instead does. `Window.Close` and its owner going close it without asking.
- `nsgo.SetMainView` sets the main view: it opens the main window at `nsgo.WindowSize` and makes it main, and a later call replaces the view in it. `nsgo.Run(Root)` is `SetMainView(Root)`.
- `OnReopen` hears a click on the Dock icon and a second launch (`EventReopen`); without a handler the main window shows and takes the focus, on Windows and Linux the app's one window, restored if minimized. Windows' named mutex and pipe and Linux's Unix socket, beside its lock in `$XDG_RUNTIME_DIR` (in a Flatpak, `$XDG_RUNTIME_DIR/app/$FLATPAK_ID`, which the app's instances share) or else in a private directory of the user's in the temporary one, hand a second launch to the running process before it creates a second engine, which dispatches `OnReopen` and its file arguments to `OnOpenFiles`; on Windows the second launch lets the running process take the foreground before it hands over.
- `OnOpenFiles` hears files the system opens with the app (`EventOpenFiles`): on macOS `application:openURLs:` for the types `file_types` registers, on Windows and Linux the files named on the command line. Files come before the app's first view builds when they launched it, so the core keeps those until a handler registers and posts them to it.
- `nsgo.HideDock` starts the app as an accessory app, before AppKit's loop runs, so its icon never shows in the Dock; `SetDockVisible` changes the activation policy later, as apps do while their settings show. macOS runs one instance of a bundle and reopens it.
- `SetLaunchAtLogin` and `LaunchAtLogin` register and read `SMAppService.mainAppService`, which takes only apps in a bundle.

## macOS

`Run` starts the engine on a surface in no window (`own`), which applies the app-level items: it opens surfaces, adds tray icons, registers shortcuts, sets the activation policy and the login item.

- A window is an `NsgoWindow` centered at its content size. AppKit's close asks the delegate, which reports `EventWindowClose` and declines; `WindowDestroy` orders the window out and closes it once the batch returned.
- A panel is an `NsgoPanel`: an `NSPanel` that can become key without activating the app, titled for the system's rounded corners and shadow, with a transparent title bar and no controls, at the status level on every space. It shows beside the rectangle `ShowAt` gave it, else under its tray icon's button, clamped to the screen's visible frame, else in the upper middle of the screen with the mouse, as Spotlight. With `HideOnBlur` it hides as it resigns key and reports `EventWindowHidden`. With `NoActivate` it is an `NsgoStillPanel`, which never becomes key or main and orders front without `makeKeyAndOrderFront:`, its views taking the first click.
- A tray icon is an `NSStatusItem`: its button reports left and right clicks, a Control-click as secondary. The click that hides a panel by taking its focus does not show it again. Its menu is the status item's own, set for one `performClick:` and reported closed as a context menu.
- A drawn icon is a surface in the button: an embedded surface whose hit test lets clicks through to the button, the item as wide as the view.
- Global shortcuts are Carbon's `RegisterEventHotKey` on the application's event target, which needs no accessibility permission; a key names its ANSI virtual key code, and one without a code is unsupported. Carbon refuses a combination the app registered already, or another app registered exclusively, with `eventHotKeyExistsErr`, which is taken; it does not see other apps' ordinary registrations, so such a combination registers and the press goes to one of them.
- An alert asked from the app's own surface, which has no window to hold a sheet, runs app-modal.

## Linux

An app run with a function (`nsgo.Run(App)`, or a root view with automatic update checks) runs on a GTK window that never shows, as macOS's `own`, and its lifecycle is its windows': active while one has the focus, in the background while none shows. An app run with a root view shows it in the one window, where `NewTray` still works.

- A window is a `GtkWindow` on the same main loop. Its `close-request` reports `EventWindowClose` and declines; `WindowDestroy` deletes its views, destroys it and drops its handle.
- A panel is a borderless window that takes the keyboard focus and with `HideOnBlur` hides as it loses it, reporting `EventWindowHidden`; GTK places it where the compositor does, not under its tray icon. On X11 `ShowAt` moves its X window once it shows, and a panel with `NoActivate` is an override-redirect window, as menus are, which no window manager decorates, lists or focuses, placed before it maps, at the upper middle of the primary display without `ShowAt`. A Wayland compositor places every toplevel and focuses a panel as it shows, so there `ShowAt` reports that it cannot place the panel; edge-docked panels on a layer-shell surface remain ([roadmap](../roadmap.md#application-and-windows)).
- A tray icon is a StatusNotifierItem on a session bus connection of its own, so that several have the protocol's `/StatusNotifierItem` path and a bus name each, closing which takes the icon away; a watcher, Plasma's or GNOME's AppIndicator extension's, shows it, and without one nothing does. It names the app's icon only without a PNG of its own, which hosts would otherwise show instead, and signals each change of its icon, title and tooltip.
- Its menu is a `com.canonical.dbusmenu` object at `/Menu`: the host's `AboutToShow` asks the app for the menu as a secondary click does and is answered once it comes, or after 250 ms with the one the icon has. Every entry has an ID of its own that later menus never reuse, and a click on one reports `EventMenu` and the menu closed. A drawn icon is not drawn yet.
- Global shortcuts are passive `XGrabKey` grabs on the X11 root window, made inside GDK's error trap so that a combination another client holds is taken rather than ending the app; GdkX11Display's `xevent` signal takes their presses, as GTK 4 has no display filters. A press matches on Shift, Control, Alt and Super alone, whatever its locks, buttons and layout group.
- On Wayland, and on X11 where no grab succeeds, they use the XDG `GlobalShortcuts` portal, where the compositor asks the user to bind them: the shortcuts are registered once `BindShortcuts` answers, taken when it is refused, and unsupported when the portal has no session; a combination another X11 client grabbed stays taken there, and a second registration of one combination is taken without going to the portal. The portal knows an unsandboxed app by the ID it registers (`Registry.Register`), which must name its `.desktop` entry and comes before any other portal call on the connection; GTK calls the portal as it starts, so the shortcuts have a session bus connection of their own, whose calls run in order off the main thread.
- Both paths dispatch the same `EventHotKey`. A host's `NewView` takes none.

## Windows

For `nsgo.Run(App)`, `Run` keeps its root window hidden as the app's own surface and attaches the surfaces the app opens on its message loop; `nsgo.Run(Root)` shows the main view in the root window and only adds tray icons.

- A window is a top-level window centered at its content size in its monitor's scale, with its material. Its close reports `EventWindowClose`, and `WindowDestroy` destroys it. Keys go to the key bindings and menu of the window they are sent to.
- A panel is a borderless popup tool window, so it has no taskbar button. It shows beside the rectangle `ShowAt` gave it in the work area of the monitor that holds it, else above or under its tray icon from `Shell_NotifyIconGetRect`, else in the upper middle of the monitor with the pointer, sized at that monitor's scale, and takes the focus, which activates the app; with `HideOnBlur` it hides as it loses it. The press on the icon that hides a panel by taking its focus does not show it again. With `NoActivate` it is `WS_EX_NOACTIVATE` and topmost, shows with `SW_SHOWNA` and answers `WM_MOUSEACTIVATE` with `MA_NOACTIVATE`, so neither showing nor a click takes the foreground.
- A tray icon is `Shell_NotifyIcon`'s at `NOTIFYICON_VERSION_4`: `NIN_SELECT` and `NIN_KEYSELECT` are primary clicks and `WM_CONTEXTMENU` secondary, its tooltip is the standard one, and the icons are added again when Explorer broadcasts `TaskbarCreated`. Its menu is owned by a hidden window, so it raises none of the app's. A drawn icon's surface stays hidden.
- Global shortcuts are `RegisterHotKey` on the hidden root window, with `MOD_NOREPEAT`; `WM_HOTKEY` reports the registered ID, and the whole set is replaced whenever it changes. A combination any window or thread on the desktop holds, the app's own included, fails with `ERROR_HOTKEY_ALREADY_REGISTERED` and is taken; a key without a virtual key code is unsupported.
- An alert asked from the app's own surface shows the hidden root window as the alert's own window, without the app's menu bar, which closing cancels.
- `LaunchAtLogin` answers false ([roadmap](../roadmap.md#application-and-windows)).

## Configuration

`nsgo.yaml` keeps what the package needs before any code runs: the name, the ID, the version and the launch screen's appearance. Windows are sized and limited in Go, `nsgo.WindowSize` and `nsgo.WindowConstraints` for the main window and `WindowOptions` for others; `nsgo.HideDock` and the shutdown mode are Go's too. A `window` in `nsgo.yaml` fails the tool, naming `nsgo.WindowSize`.

## Departures from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| `run_app` shows one `AppView` in one window; more windows are a host's, an engine each | One engine with a surface per window, panel and tray view, which the app opens as Avalonia's apps do | Go code is trusted and in one process, so windows share signals instead of crossing a host channel |
| `keep_alive_on_close`, macOS only | `SetShutdownMode` and a cancelable `OnClosing` | Tray and background apps outlive their windows |
| No tray, panels, global shortcuts, Dock policy or launch at login | `desktop` has them | Desktop apps need them |
| `nsh.json`'s window size | `nsgo.WindowSize`, `nsgo.WindowConstraints` and `WindowOptions` | An app opens windows of different sizes, or none |

## Verification

`core/window_test.go` opens windows, panels and tray icons on a platform that opens windows, headless: shared signals, shutdown modes, canceled closes, reopening, keys and owners, tray clicks and menus, shortcuts, the Dock and the login item, and the main view where no windows open. `internal/engine` attaches an opened window and destroys it, and the Windows backend's tests decode the notification area's callbacks. On macOS `examples/tray` was run: its welcome window, its panel under the icon after a click, a close that keeps the app, and the gallery ending with its window. A panel placed with `ShowAt` and `MoveTo` was run on macOS 27, Windows 11 and Ubuntu 24.04 on Xwayland (2026-11-16): beside, flipped above at the bottom of the display and at a point; with `NoActivate` the app in front kept the foreground through its show, a click on its button that ran the handler, its moves and a toggle, while a panel without it took the keyboard; on Wayland `ShowAt` reported that it could not place the panel. `driver.PlaceWindow`'s flips and clamps have a test of their own.
