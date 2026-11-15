---
title: Platform services
description: Alerts, the clipboard, files, photos, sharing, storage, speech, networking, signing in and the app's lifecycle.
group: Platform features
order: 44
---

## Alerts and the clipboard

`cx.ShowAlert` shows the platform's alert or action sheet. Its `Actions` come back by index, and a role draws the cancel and destructive buttons the platform's way.

```go
cx.ShowAlert(core.AlertConfig{
	Title:   "Delete the file?",
	Message: "This cannot be undone.",
	Actions: []core.AlertAction{
		{Label: "Cancel", Role: core.AlertCancel},
		{Label: "Delete", Role: core.AlertDestructive},
	},
}, func(index int) {
	if index == 1 {
		remove()
	}
})
```

`SetClipboard` writes text and `SetClipboardImage` an image. `ReadClipboard`, `ReadClipboardImage` and `ReadClipboardFiles` answer asynchronously, and `HasClipboardText` and `HasClipboardImage` read what the clipboard holds without a paste notice. `core.FileFromURL` opens the URLs `ReadClipboardFiles` and `View.OnFileDrop` give as a `core.File`: `file:` URLs, and on the web `blob:` URLs.

```go
cx.ReadClipboard(func(text string) { pasted.Set(text) })
cx.SetClipboard("Hello")
```

## Opening links and files

`cx.OpenURL` opens a URL in the system's browser or app, and `core.CheckURL` says what it refuses: no scheme, over 16 KiB, spaces, a web address without a host, and `javascript:`, `data:`, `blob:`, `file:`, `about:` and `asset:`.

```go
if err := core.CheckURL(url); err == nil {
	cx.OpenURL(url, func(opened bool) {
		if !opened {
			status.Set("No app opens that link.")
		}
	})
}
```

`cx.OpenFile` opens a file the app has in the app associated with its type, and `cx.Share` shows the system's share sheet for files, text or both. A `core.SharedFile` is a path, with the name and type the other app sees. On desktops, `desktop.RevealFile` shows a file or folder selected in Finder, Explorer or the file manager, and `desktop.TrashFile` moves it to the Trash or Recycle Bin; it never deletes permanently, answering `core.ErrNoTrash` and leaving the item in place where the volume has no trash or the trash would not keep it, such as a file larger than the Recycle Bin.

```go
cx.Share(core.ShareOptions{
	Files: []core.SharedFile{{Path: path, Name: "Notes.txt"}},
	Text:  "A note from NSGo",
}, func(shown bool, err error) {})
```

## Photos and files

`cx.PickImages` shows the system's photo picker and answers with `core.PickedFile`s, whose bytes, name, type and pixel size the app reads. `cx.TakePhoto` shows the camera where the platform has one; `cx.Env().CanTakePhoto` says whether it does.

```go
cx.PickImages(core.PickOptions{Limit: 1, MaxDimension: 2048}, func(files []core.PickedFile) {
	if len(files) == 1 {
		avatar.Set(files[0].Data)
	}
})
```

`cx.PickFiles` picks files of any type, `cx.PickDirectories` picks folders as an `fs.FS`, and `cx.SaveFile` saves a file or bytes through the system's save dialog. Both pickers take a `Title`, a `Button` label and a `Start` folder, and `PickFilesOptions.TypesName` names the type menu, so an app can show them in its own language; these are hints, which macOS, Windows and Linux follow, iOS only for `Start`, and Android and the web not at all. `cx.PickScreenColor` picks a color anywhere on the screen where the platform allows it.

## Storage

`cx.Storage()` is the app's key-value storage, kept on the device and shared by every graph of the app. Reads are synchronous and read inside a computation subscribe it to changes; `SetItem` returns a `core.StorageFullError` past the quota (`core.StorageQuota`, 5 MiB by default).

```go
store := cx.Storage()
if token, ok := store.GetItem("token"); ok {
	login(token)
}
store.SetItem("token", token)
```

`store.Keys()` lists the keys, `RemoveItem` deletes one and `Clear` empties the storage. `cx.DataDir()` and `cx.CacheDir()` give the app's folders for files the key-value storage is too small for; `cx.LogsDir()` gives the platform's directory for log files, and a `Scope.ProvideStorage` gives a subtree a partition of its own.

## Networking

Go's own `net/http` makes requests on every target, with the system's TLS and proxy settings. Signals belong to the engine's goroutine, so a response from another goroutine reaches them through `cx.Post`.

```go
go func() {
	resp, err := http.Get("https://api.example.com/items")
	if err != nil {
		cx.Post(func() { errText.Set(err.Error()) })
		return
	}
	defer resp.Body.Close()
	var items []Item
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		cx.Post(func() { errText.Set(err.Error()) })
		return
	}
	cx.Post(func() { list.Set(items) })
}()
```

`cx.Post` runs its function on the graph's goroutine, where signals may be set. On Android the manifest asks for the internet permission, and image downloads cache themselves on disk.

`core.NewResource` does the same for a value loaded from a source. Its fetcher runs on a goroutine of its own and loads again when the source changes:

```go
items := core.NewResource(cx, func() (string, bool) { return query.Get(), query.Get() != "" },
	func(ctx context.Context, q string, _ core.ResourceInfo) ([]Item, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/items?q="+url.QueryEscape(q), nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var list []Item
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			return nil, err
		}
		return list, nil
	})
```

`items.Get()`, `items.Err()` and `items.Loading()` are signals that bindings follow. A new query cancels the request still running through `ctx`, so a late answer never replaces a newer one. `items.Refetch()` retries, keeping the last list while it loads, and `items.Mutate` sets a value at once for an optimistic update.

`core.Suspense` shows a fallback while the resources its content reads load their first value, so a screen needs no loading flag of its own:

```go
core.Suspense{
	Content:  func(cx *core.Scope) core.Node { return itemList(cx, items) },
	Fallback: func(*core.Scope) core.Node { return ui.Spinner{Label: "Loading"} },
}
```

The content stays mounted but hidden while it waits, so it shows at once when the values arrive. A refetch keeps it shown with the last value.

## Speech and the lifecycle

`cx.StartSpeech` recognizes speech with the system's recognizer where `cx.Env().CanRecognizeSpeech()` says it exists, and answers with the transcript as it changes.

```go
session, err := cx.StartSpeech(core.SpeechOptions{
	Lang:    "en-US",
	Interim: true,
	OnResult: func(text string, final bool) {
		transcript.Set(text)
	},
})
if err == nil {
	defer session.Stop()
}
```

`cx.Env().Lifecycle()` says whether the app is active, inactive or in the background: on desktops the window `cx` is in, so a minimized window's content can pause while another window shows, and in the app's own scope all its windows together. `desktop.Minimized(cx)` says whether that window is minimized. `cx.OnQuit` cleans up as it quits, however it does, with 200 ms for its work. `desktop.OnQuitRequest` hears a quit the user asks for on macOS, such as Cmd+Q, and may cancel it or defer it while it asks the user. `cx.SetAppearance` forces light or dark, `cx.SetLocale` chooses the language, `cx.OnKey` binds a keyboard shortcut, which rests while what its owner shows is hidden or under a dialog unless `core.KeyOptions{OverModals: true}` keeps it, and which leaves a focused control the keys it uses, a button its Return and Space and a slider, tab bar or radio group its arrows, unless bound with `core.KeyAlways` or `core.KeyNotEditing`; `cx.OnOpenFiles` hears the documents a macOS app opens.

```go
cx.OnKey("mod+k", func() { palette.Set(true) })
cx.SetLocale("de")
cx.OnQuit(func(ctx context.Context) { saveDraft(ctx) })
```

`desktop.RegisterHotKey` binds a global shortcut, which works while any app is frontmost, on macOS, on Linux and in a Windows app run with `nsgo.Run(App)`. Its handle's `State` says whether the system took it: `HotKeyPending` until it answers, then `HotKeyRegistered`, `HotKeyTaken` when another registration holds the combination, or `HotKeyUnsupported` for a key or a platform without global shortcuts, so a shortcut recorder can ask for another combination. On macOS a combination counts as taken only when the app holds it already or another app registered it exclusively. On Wayland the compositor asks the user to bind the shortcut, and it is registered once they have. In tests, `nsgotest.HotKeyTaken` and `nsgotest.HotKeyUnsupported` make combinations answer so.

```go
shortcut := desktop.RegisterHotKey(cx, "alt+space", launcher.Toggle)
warning := core.Text{ContentFunc: func() string {
	if shortcut.State() == desktop.HotKeyTaken {
		return "Another app uses Alt+Space"
	}
	return ""
}}
```

## Biometrics

`cx.BiometricAvailability` reports whether Face ID, Touch ID or a device fingerprint is enrolled; Android does not say which kind is enrolled, so there it reports a fingerprint when one is enrolled and otherwise face when the device has face hardware. `cx.AuthenticateBiometric` asks the system to confirm the user with a reason, which must not be empty, and reports success, failure, cancellation or unavailability. It does not choose a passcode fallback for the app.

```go
cx.BiometricAvailability(func(kind core.BiometricKind) {
	if kind != core.BiometricNone {
		cx.AuthenticateBiometric("Confirm deleting this account", func(result core.BiometricResult) {
			if result == core.BiometricSuccess {
				deleteAccount()
			}
		})
	}
})
```

Face ID apps declare `NSFaceIDUsageDescription` under `apple.info`; Android apps list `USE_BIOMETRIC` under `android.permissions`.

## Signing in

`cx.AuthSession` signs the user in with an OAuth provider in the system's authentication session rather than a web view: `ASWebAuthenticationSession` on macOS and iOS, the default browser with a loopback redirect on Windows, Linux and a Mac without a scheme, and a popup on the web. The app keeps the protocol: it makes the PKCE verifier and the state, builds the authorization URL for the redirect it is given, and checks the callback before trading its code. Cancelling is not failing: `errors.Is(err, core.AuthCancelled)` tells the user closing the session, or the returned `cancel` ending it.

```go
verifier, challenge, state := newPKCE() // the app's own
cx.AuthSession(func(redirect string) string {
	q := url.Values{"client_id": {clientID(redirect)}, "redirect_uri": {redirect}, "response_type": {"code"}, "scope": {"openid email"},
		"state": {state}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	return "https://accounts.example.com/authorize?" + q.Encode()
}, core.AuthSessionOptions{Scheme: "com.example.app", Path: "/oauth2redirect", Timeout: 5 * time.Minute}, func(callback string, err error) {
	if errors.Is(err, core.AuthCancelled) {
		return
	}
	if err != nil {
		showError(err)
		return
	}
	u, _ := url.Parse(callback)
	if u.Query().Get("state") == state {
		exchangeCode(u.Query().Get("code"), verifier)
	}
})
```

The redirect is `com.example.app:/oauth2redirect` where the system's session catches the scheme, which the app need not declare, and `http://127.0.0.1:<port>/oauth2redirect` on the desktops' loopback, so register both with the provider, often as clients of their own. Leave `Scheme` empty for loopback alone, or set it to `https` with `Host` for the app's associated domain, which iOS 17.4 and macOS 14.4 catch. A closed browser tab tells a loopback session nothing, so give it a `Timeout` or a cancel button. `Ephemeral` asks macOS and iOS not to share the browser's cookies. Android, tvOS, watchOS and mini programs answer `core.AuthUnavailable` for now. In TSX, `AuthSession.start(url, options)` needs the `auth_session` capability and gives a session whose `result` resolves with the callback, or null once cancelled.

## Overlays on Android

`mobile.OpenOverlay` shows a view of the app's over other apps on Android, a tree of its own that shares the app's signals, and a foreground service keeps the app running while it shows. The app lists `SYSTEM_ALERT_WINDOW` under `android.permissions`, and the user grants it in the system's settings, which `mobile.RequestOverlayPermission` opens; without it the overlay ends at once and `OnEnd` hears why. It takes touches but leaves the keys and the touches around it to the app in front, unless it is `Focusable` or `Passive`, and moves with `desktop.SetWindowFrame` from its content. Other platforms open none.

```go
mobile.RequestOverlayPermission(cx, func(p core.OverlayPermission) {
	if p == mobile.OverlayGranted {
		mobile.OpenOverlay(cx, mobile.OverlayOptions{Width: 200, Height: 80}, Score)
	}
})
```

## Next

- [Notifications](notifications.md): permission, badges and presses.
- [Forms](forms.md): text fields and the keyboard.
- [Embedding](embedding.md): services a host app gives its embedded app.
- [Building and shipping](building.md): the permissions and files a package needs.
