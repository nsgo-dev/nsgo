# Changelog

## [Unreleased]

- Add HarmonyOS as a preview target, whose ArkTS host runs the engine in ArkWeb and draws it with ArkUI views, in `nsgo build`, `nsgo run` and `nsgo test -target harmonyos` (#79 by @togotop).
- Add in-app purchases on macOS, iOS and tvOS through StoreKit 2: the store's products and prices, purchases answering cancelled and pending apart from failed with the signed transaction, updates made outside the app, finishing, current entitlements and restoring, in Go, TSX and `nsgotest`, when `nsgo.yaml` sets `apple.purchases` (#353 by @houko).
- Add `desktop.SetWindowWorkspaces` and `WindowOptions.Workspaces`, which keep a window on every Space and over other apps' full screen spaces on macOS, at any level but the desktop's, which stays behind them, and on every virtual desktop on X11, with `desktop.CanSetWindowWorkspaces`, which says whether it takes effect (#572 by @houko).
- Add transparent desktop windows without a frame, shadow or taskbar button, as `WindowOptions.Transparent`, `NoShadow` and `SkipTaskbar`, the main window's `nsgo.WindowTransparent`, `WindowNoShadow` and `SkipTaskbar`, and `desktop.SetWindowTransparent`, `SetWindowShadow` and `SetSkipTaskbar`, on macOS, Windows and Linux, with `desktop.CanMakeWindowsTransparent` and `CanSkipTaskbar` saying where they take effect (#567 by @houko).
- Add `SidebarShell.Fixed`, a sidebar the user cannot resize.
- Add `desktop.SetWindowPointerEvents`, with `WindowOptions.PointerEvents` and `PanelOptions.PointerEvents`, which lets the pointer through a window to the windows below it everywhere or wherever it draws nothing, on macOS, Windows and Linux (#568 by @houko).
- Add `desktop.FrontmostApp`, the application in front on the desktop with its ID, name, process ID and full screen, tracked as other apps come to the front, on macOS, Windows and X11, and `desktop.CanReadFrontmostApp`, which says where it is followed; both are held to `core.AccessFrontmostApp` and set in tests through `nsgotest.Environment` (#570 by @houko).
- Add `ImageSource.Pixels` and `Version`, which show an `image.RGBA` the app draws in an `Image` or a `Canvas` with its alpha, no load events and no fade, swapped in place without encoding on Apple platforms, Windows and Linux (#107 by @houko).
- Add `Scope.SetWebCookies`, which writes cookies, HttpOnly ones included, to the persistent store the app's web views share on Apple platforms, Windows, Linux and Android, so they can carry the sign-in of the user's own browser (#387 by @houko).
- Add `desktop.CanSetWindowLevel` and `desktop.CanPlaceWindows`, and keep a floating window above the others on Linux under X11, so a Wayland app can tell that its window is neither stacked nor placed (#573 by @houko).
- Add `WebView.PinchZoom`, `Ref.Load` and the platform's `Domain` and `Code` on `WebError`; on Windows, Ctrl with the wheel or with + and - zooms a web view only with `PinchZoom` (#233 by @houko).
- Report `Video.OnError` instead of an opaque picture for a local video whose alpha the platform cannot keep: WebM with alpha on macOS and iOS, any clip with alpha on Windows until Media Foundation's alpha output is checked, and on Linux HEVC with alpha, or VP8 and VP9 with alpha without GStreamer's codecalpha plug-in; add `MediaError.Reason`, `MediaErrorAlphaUnsupported` for these, so an app can fall back to its own decoder without matching the message (#569 by @houko).
- Stop reporting a web view load that was stopped, replaced or turned into a download to `OnError` on Apple platforms and Windows (#233 by @houko).
- Keep a view's own outer shadows and outline outside its `Overflow: Hidden` clip on Apple for views, images and macOS scroll views, as CSS does (#238 by @houko).
- Show a bundled macOS app's Dock badge without the notification permission until the app uses notifications (#190 by @houko).

## [0.1.0] - 2026-11-16

NSGo 0.1.0 is the first release of a cross-platform native UI framework in Go, porting NSH's design: write an app once, in Go or TSX, and it runs on phones, tablets, TVs, watches, desktops and the web in each platform's own views. Its API is not stable yet.

- **Every platform from one Mac.** Build for macOS, iOS with App Clips, tvOS, watchOS, Windows, Linux, Android and the web with only Go and Xcode, and for WeChat, QQ, Alipay and Douyin mini programs experimentally.
- **Native views.** Text, input methods, scrolling, menus and accessibility are each platform's own, and what one platform has, such as Liquid Glass, the others imitate.
- **Fine-grained updates.** Signals update only what reads them, through flexbox and grid layout, CSS-like styles, Tailwind classes, themes and animations on the platform's frame clock.
- **Components.** `base` offers unstyled components from buttons to a million-row `DataTable`, charts, flow diagrams, a code editor, a rich-text editor and a terminal, and `ui` gives them NSGo's look.
- **Platform services.** Use navigation, notifications, permissions, pickers, sharing, haptics, widgets and Live Activities, and on desktops windows, panels, menus, tray icons and global shortcuts.
- **TSX apps.** Write apps in TSX as NSH does, with hot updates that keep state, and ship them as signed, sandboxed packages a Go app runs.
- **Localized and embeddable.** Translate with YAML catalogs, CLDR plurals and right-to-left layout, and embed NSGo in iOS, Android, Go desktop and web apps.
- **The `nsgo` tool.** Create apps, run them with hot reload and DevTools, test them with `nsgotest`, and package them for app stores, Flathub and direct downloads with updates.
- **Small and fast.** A counter app packs to about 8 MB on the desktops and starts faster than NSH, MyGo and GPUI in NSGo's benchmarks.

Thanks to everyone who shaped this release with code, reports and ideas: @houko, @daodao97, @Heyana, @ayuayue, @henry-wm, @Jungley8, @togotop, @westhiker, @Laotree, @OfficialDiLi, @happyhaha1, @roamlog, @yangjuncode, @pdenapo and @monkeyWie.
