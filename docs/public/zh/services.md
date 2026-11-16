---
title: 平台服务
description: 警告框、剪贴板、文件、照片、分享、存储、语音、网络、登录和应用生命周期。
group: 平台功能
order: 44
---

## 警告框与剪贴板

`cx.ShowAlert` 显示平台的警告框或操作表。`Actions` 按下标返回，角色会让平台用对应方式绘制取消和危险按钮。

```go
cx.ShowAlert(core.AlertConfig{
	Title:   "删除这个文件？",
	Message: "此操作无法撤销。",
	Actions: []core.AlertAction{
		{Label: "取消", Role: core.AlertCancel},
		{Label: "删除", Role: core.AlertDestructive},
	},
}, func(index int) {
	if index == 1 {
		remove()
	}
})
```

`SetClipboard` 写入文本，`SetClipboardImage` 写入图片。`ReadClipboard`、`ReadClipboardImage` 和 `ReadClipboardFiles` 异步回答，`HasClipboardText` 和 `HasClipboardImage` 只读取剪贴板内容，不会触发粘贴提示。`core.FileFromURL` 把 `ReadClipboardFiles` 和 `View.OnFileDrop` 给出的 URL 打开为 `core.File`：`file:` URL，以及 Web 上的 `blob:` URL。

```go
cx.ReadClipboard(func(text string) { pasted.Set(text) })
cx.SetClipboard("Hello")
```

## 打开链接与文件

`cx.OpenURL` 用系统的浏览器或应用打开链接，`core.CheckURL` 说明它会拒绝什么：没有 scheme、超过 16 KiB、包含空格、Web 地址没有 host，以及 `javascript:`、`data:`、`blob:`、`file:`、`about:` 和 `asset:`。

```go
if err := core.CheckURL(url); err == nil {
	cx.OpenURL(url, func(opened bool) {
		if !opened {
			status.Set("没有应用能打开这个链接。")
		}
	})
}
```

`cx.OpenFile` 用系统关联的应用打开应用持有的文件，`cx.Share` 显示系统分享面板，可以分享文件、文本或两者。`core.SharedFile` 是一个路径，以及另一个应用看到的名称和类型。在桌面上，`desktop.RevealFile` 在访达、资源管理器或文件管理器中选中显示一个文件或文件夹，`desktop.TrashFile` 把它移到废纸篓或回收站；它从不永久删除，卷没有废纸篓或废纸篓容不下该项目（例如比回收站还大的文件）时回答 `core.ErrNoTrash`，项目留在原处。

```go
cx.Share(core.ShareOptions{
	Files: []core.SharedFile{{Path: path, Name: "Notes.txt"}},
	Text:  "来自 NSGo 的笔记",
}, func(shown bool, err error) {})
```

## 照片与文件

`cx.PickImages` 显示系统照片选择器，返回 `core.PickedFile`，其中包含字节、名称、类型和像素尺寸。`cx.TakePhoto` 在平台有相机时打开相机；`cx.Env().CanTakePhoto` 说明是否有。

```go
cx.PickImages(core.PickOptions{Limit: 1, MaxDimension: 2048}, func(files []core.PickedFile) {
	if len(files) == 1 {
		avatar.Set(files[0].Data)
	}
})
```

`cx.PickFiles` 选择任意类型的文件，`cx.PickDirectories` 把文件夹选成 `fs.FS`，`cx.SaveFile` 通过系统保存对话框保存文件或字节。两种选择器都可以设置 `Title`（标题）、`Button`（按钮文字）和 `Start`（起始文件夹），`PickFilesOptions.TypesName` 还能命名类型菜单，便于用应用自己的语言显示；这些只是提示，macOS、Windows 和 Linux 会遵循，iOS 只支持 `Start`，Android 和 Web 都会忽略。`cx.PickScreenColor` 在平台允许时从屏幕任意位置取色。

## 存储

`cx.Storage()` 是应用的键值存储，保存在设备上，应用的每个响应式图共享它。读取是同步的，在计算中读取会让它订阅变化；超过配额时 `SetItem` 返回 `core.StorageFullError`（默认 `core.StorageQuota` 为 5 MiB）。

```go
store := cx.Storage()
if token, ok := store.GetItem("token"); ok {
	login(token)
}
store.SetItem("token", token)
```

`store.Keys()` 列出键，`RemoveItem` 删除一个，`Clear` 清空存储。`cx.DataDir()` 和 `cx.CacheDir()` 给出应用存放键值存储放不下的文件的目录；`Scope.ProvideStorage` 给子树一个独立分区。

## 网络

Go 自带的 `net/http` 在每个目标上都能发起请求，使用系统的 TLS 和代理设置。signal 属于引擎的 goroutine，因此其他 goroutine 得到的响应要通过 `cx.Post` 交给它。

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

`cx.Post` 在响应式图的 goroutine 上运行函数，因此可以在那里设置 signal。Android 的清单默认申请网络权限，图片下载会缓存在磁盘上。

从某个来源加载值时，`core.NewResource` 会替你完成这些：它的 fetcher 在自己的 goroutine 上运行，来源变化时重新加载：

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

`items.Get()`、`items.Err()` 和 `items.Loading()` 都是 signal，绑定会跟着它们更新。新的查询会通过 `ctx` 取消仍在进行的请求，迟到的响应不会覆盖较新的结果。`items.Refetch()` 用于重试，加载期间保留上一次的列表；`items.Mutate` 立即设置一个值，用于乐观更新。

`core.Suspense` 会在其内容读取的资源加载首个值时显示占位内容，页面无需自己维护加载状态：

```go
core.Suspense{
	Content:  func(cx *core.Scope) core.Node { return itemList(cx, items) },
	Fallback: func(*core.Scope) core.Node { return ui.Spinner{Label: "Loading"} },
}
```

等待期间内容保持挂载但处于隐藏状态，值一到就能立即显示。重新加载时会继续显示上一次的值。

## 语音与生命周期

`cx.StartSpeech` 在 `cx.Env().CanRecognizeSpeech()` 为真时用系统识别器识别语音，并随着转写结果变化返回文本。

```go
session, err := cx.StartSpeech(core.SpeechOptions{
	Lang:    "zh-CN",
	Interim: true,
	OnResult: func(text string, final bool) {
		transcript.Set(text)
	},
})
if err == nil {
	defer session.Stop()
}
```

`cx.Env().Lifecycle()` 说明应用处于前台、非活动还是后台：在桌面上是 `cx` 所在窗口的状态，所以一个窗口最小化时其内容可以暂停，而其他窗口照常显示；在应用自身的作用域里则是所有窗口合起来的状态。`desktop.Minimized(cx)` 说明该窗口是否最小化。`cx.OnQuit` 在应用退出时做清理，无论以何种方式退出，最多有 200 毫秒完成工作。`desktop.OnQuitRequest` 在 macOS 上听到用户发起的退出，例如 Cmd+Q，可以取消它，或推迟到询问过用户再答复。`cx.SetAppearance` 强制浅色或深色，`cx.SetLocale` 选择语言，`cx.OnKey` 绑定键盘快捷键，它的所有者显示的内容被隐藏或被对话框盖住时不生效，除非用 `core.KeyOptions{OverModals: true}` 保留；获得焦点的控件自己要用的键也留给控件，按钮的 Return 和 Space，滑块、标签栏和单选组的方向键，除非用 `core.KeyAlways` 或 `core.KeyNotEditing` 绑定；`cx.OnOpenFiles` 听到 macOS 应用打开的文件。

```go
cx.OnKey("mod+k", func() { palette.Set(true) })
cx.SetLocale("de")
cx.OnQuit(func(ctx context.Context) { saveDraft(ctx) })
```

`desktop.RegisterHotKey` 绑定全局快捷键，无论哪个应用在前台都生效，支持 macOS、Linux，以及用 `nsgo.Run(App)` 运行的 Windows 应用。它返回的句柄的 `State` 说明系统是否接受了它：系统答复前是 `HotKeyPending`，之后是 `HotKeyRegistered`；组合键已被其他注册占用时是 `HotKeyTaken`；按键或平台不支持全局快捷键时是 `HotKeyUnsupported`。快捷键录制器据此可以请用户换一个组合。在 macOS 上，只有应用自己已注册、或其他应用以独占方式注册的组合键才算被占用。在 Wayland 上由合成器请用户绑定快捷键，用户绑定后才是已注册。测试中用 `nsgotest.HotKeyTaken` 和 `nsgotest.HotKeyUnsupported` 让组合键这样答复。

```go
shortcut := desktop.RegisterHotKey(cx, "alt+space", launcher.Toggle)
warning := core.Text{ContentFunc: func() string {
	if shortcut.State() == desktop.HotKeyTaken {
		return "Alt+Space 已被其他应用使用"
	}
	return ""
}}
```

## 登录

`cx.AuthSession` 在系统的认证会话里用 OAuth 服务登录，而不是在网页视图里：macOS 和 iOS 用 `ASWebAuthenticationSession`，Windows、Linux 以及没有设置 scheme 的 Mac 用默认浏览器加回环地址重定向，Web 用弹出窗口。协议由应用负责：它生成 PKCE 校验码和 state，为传给它的重定向地址拼出授权 URL，并在用 code 换令牌前检查回调。取消不算失败：`errors.Is(err, core.AuthCancelled)` 表示用户关闭了会话，或返回的 `cancel` 结束了它。

```go
verifier, challenge, state := newPKCE() // 应用自己的实现
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

系统会话能捕获 scheme 的平台上，重定向地址是 `com.example.app:/oauth2redirect`，应用不必声明这个 scheme；桌面的回环地址上是 `http://127.0.0.1:<端口>/oauth2redirect`，所以两者都要在服务方登记，通常各是一个客户端。`Scheme` 留空则只用回环地址；设为 `https` 并给出 `Host` 则用应用的关联域名，iOS 17.4 和 macOS 14.4 起能捕获。浏览器标签页被关闭时回环会话无从得知，所以要给它 `Timeout` 或一个取消按钮。`Ephemeral` 让 macOS 和 iOS 不共享浏览器的 Cookie。Android、tvOS、watchOS 和小程序目前回答 `core.AuthUnavailable`。在 TSX 中，`AuthSession.start(url, options)` 需要 `auth_session` 能力，返回的会话的 `result` 解析为回调地址，取消时为 null。

## Android 上的悬浮窗

`mobile.OpenOverlay` 在 Android 上把应用的一个视图显示在其他应用之上：它是一棵独立的树，与应用共享信号，显示期间由前台服务让应用保持运行。应用要在 `android.permissions` 中列出 `SYSTEM_ALERT_WINDOW`，并由用户在系统设置中授权，`mobile.RequestOverlayPermission` 会打开这个设置；没有授权时悬浮窗立即结束，`OnEnd` 会得知原因。它接收触摸，但按键和窗口以外的触摸仍交给前台应用，除非设为 `Focusable` 或 `Passive`；在内容里用 `desktop.SetWindowFrame` 移动它。其他平台不会打开悬浮窗。

```go
mobile.RequestOverlayPermission(cx, func(p core.OverlayPermission) {
	if p == mobile.OverlayGranted {
		mobile.OpenOverlay(cx, mobile.OverlayOptions{Width: 200, Height: 80}, Score)
	}
})
```

## 下一步

- [通知](notifications.md)：权限、图标角标和点按。
- [表单](forms.md)：文本框和键盘。
- [嵌入](embedding.md)：宿主应用提供给嵌入应用的服务。
- [构建与发布](building.md)：应用包需要的权限和文件。
