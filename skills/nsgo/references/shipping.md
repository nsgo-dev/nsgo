# Project layout and shipping

## Layout

`go tool nsgo init` writes the layout every app keeps; `examples/hello` in the module directory is the smallest complete one.

| Path | What |
| --- | --- |
| `app.go` (or `main.go`) | The main package: `Root`, or `App` for a desktop app opening its own windows, and `main` calling `nsgo.Run` |
| `app_test.go` | `nsgotest` tests, with `TestMain` calling `nsgotest.Main` |
| `nsgo.yaml` | The app's name, id, version, icons, targets and packaging |
| `locales/` | A YAML catalog per language (`en.yaml`, `zh-Hans.yaml`) and the package the tool generates its functions into (`catalog_gen.go`); never edit generated files |
| `assets/` | Icons (`AppIcon.icon`, `Icon.png`) and what the app embeds; `assets/*-dev.*` icons mark dev builds |
| `.nsgo/` | The tool's own files, such as TSX types and test screenshots; ignore it in Git |
| `dist/` | Packages `build` writes; ignore it in Git |

Keep packages the app imports in the same module; `go tool nsgo check` and `test` take a directory without `nsgo.yaml`, or `./...`, as every package under it.

## nsgo.yaml

```yaml
name: Hello
id: dev.example.hello
version: 0.1.0
icon:
  apple: assets/AppIcon.icon   # macOS, iOS and watchOS
  png: assets/Icon.png         # every other target
dev:
  target: native               # what `nsgo dev` runs without -target
build:
  targets: [macos_arm64, windows_x64, linux_x64, ios, android_arm64, web]
  # tags: [timetzdata]         # Go build tags every build, test and check of the app takes
# i18n: { dir: locales, source: en }
# apple: { info: { NSCameraUsageDescription: Scans receipts. } }
# release: { github: me/hello }
# updates: { key: <public key from nsgo keygen> }
```

A flag replaces the value it names (`-target`, `-version`, `-output`); `go tool nsgo build -h` lists them. Read the version-matched [Building and shipping](https://nsgo.dev/docs/building.md) page for the rest: installers, store packages, signing, notarizing, GitHub releases, updates, file types, App Clips, widgets and mini programs.

## Build

```sh
go tool nsgo build                       # every target nsgo.yaml lists
go tool nsgo build -target web           # or all, macos_arm64, ios, android_arm64, miniprogram_wechat, ...
go tool nsgo build -run -device "iPhone" # build and start or install it
go tool nsgo build -channel store        # store packages instead of installers and self-updates
```

Packages land in `dist/<name>-<version>-<target>`, and everything to hand out in `dist/releases`, each file named `<name>-<version>-<system>-<arch>` with its `.sha256`. Builds are release builds; `go tool nsgo dev -release` runs one with hot reload for measuring.

## Secrets

Never commit secret keys or passwords. The tool reads them from the environment: `NSGO_UPDATES_KEY` for update feeds, `NSGO_KEYSTORE_PASSWORD` and `NSGO_KEY_PASSWORD` for Android keystores, `NSGO_NOTARY_KEY`, `NSGO_NOTARY_KEY_ID` and `NSGO_NOTARY_ISSUER` for notarizing in CI, `NSGO_ROOT_KEY` and `NSGO_RELEASE_KEY` for signed TSX packages. Public keys belong in `nsgo.yaml` and the app's code.

## Permissions and platform entries

Ask for a permission where the app uses it, through `cx`'s permission calls, and explain it in `apple.info`'s usage descriptions; on Android, `android.permissions` adds manifest permissions beyond the runtime's. An app calling `cx.SignInWithApple` sets `apple.sign_in_with_apple: true`, which signs it with the entitlement the sheet needs. Check `cx.Env()` (`CanRecognizeSpeech`, `CanSpeak`, `HasWebView` and the rest) before using a service a target may lack, rather than testing the target's name.

## Embedding

`go tool nsgo build -embed -target ios|android_arm64` packages an app as an XCFramework or an AAR for a host app of your own, `nsgo.NewView` shows one in a Go desktop program, and the web build's loader puts apps in a page. `examples/embed-*` show each host ([Embedding](https://nsgo.dev/docs/embedding.md)).
