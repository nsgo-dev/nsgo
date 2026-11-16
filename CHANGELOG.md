# Changelog

## [Unreleased]

- Add `WebView.PinchZoom`, `Ref.Load` and the platform's `Domain` and `Code` on `WebError`; on Windows, Ctrl with the wheel or with + and - zooms a web view only with `PinchZoom` (#233 by @houko).
- Stop reporting a web view load that was stopped, replaced or turned into a download to `OnError` on Apple platforms and Windows (#233 by @houko).

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
