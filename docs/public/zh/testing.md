---
title: 测试
description: 像用户一样测试应用，可以无窗口运行，也可以在原生窗口中运行。
group: 工具链
order: 21
---

## 一个测试

`nsgotest` 无需窗口即可启动应用。测试按文本或角色找到控件，并像用户一样操作它们：

```go
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}
```

当测试在某个目标上运行时，`TestMain` 会把主线程交给平台。

需要进程级初始化和清理时，把它们放在 `MainFunc` 中 `m.Run` 的前后；测试进程退出前会执行延迟清理：

```go
func TestMain(m *testing.M) {
	nsgotest.MainFunc(func() int {
		setup()
		defer teardown()
		return m.Run()
	})
}
```

`Main(m)` 是 `MainFunc(m.Run)` 的简写。目标测试中的每个测试都在独立进程运行，因此这组初始化和清理会在每个测试进程中执行一次。

用应用函数运行的应用（`nsgo.Run(func(app *core.Scope) {...})`）用 `nsgotest.StartApp(t, fn)` 测试：它像 `Run` 一样在应用自己的作用域中运行该函数一次，因此应用菜单和退出处理器都会保留。使用桌面环境的 `nsgotest.Environment` 时，凡是 `nsgo.Run` 会打开窗口的地方（macOS，以及 Windows 和 Linux 上的应用函数），应用的主视图都显示在主窗口中。此时 `desktop.CurrentWindow` 和 `OnClosing` 都能工作；`app.CloseWindow()` 发出窗口关闭按钮的请求，并返回窗口是否关闭。单独测试一个处理器时，直接调用它并读取 `WindowClosing.Canceled()`，`OnQuitRequest` 的处理器则读取 `QuitRequest.Canceled()`。步骤只作用于主窗口。`app.DropFiles` 可以指定放置区域中的任意视图：外层最近的带 `OnFileDrop` 的视图接收文件。视图的 `Bounds()` 是它在窗口坐标（点）中的框，与 `Ref.Bounds` 报告的一致。无头运行时，`app.Press` 会像在目标上一样切换系统开关（`ui.SwitchSystem`），`app.Emit("id:switch", "change", true)` 则直接报告一个值。`app.Navigate` 和 `app.PostMessage` 能到达已挂载但隐藏的 Web 视图，因为它仍会继续加载。

## 运行测试

```sh
go tool nsgo test
```

`test` 每个测试输出一行。`-run` 像 `go test` 一样筛选测试，`-junit report.xml` 写出 JUnit 报告，`-trace` 打印每一步的挂载项和提交耗时，`-bench` 改为运行基准测试，每个结果输出一行。`--` 之后的参数交给 `go test`。

传入没有 `nsgo.yaml` 的目录（例如库或模块根目录）时，`test` 会处理其下的所有包；目录也可以像 Go 的包模式那样以 `/...` 结尾。它只接受一个目录，遇到第二个会报错退出：要测试多个目录，就传入它们共同的上级目录，或逐个运行。`go tool nsgo check` 以同样的方式选取并检查包。

## 在平台上运行

同一组测试可以在真实窗口中、用平台自己的输入运行：

```sh
go tool nsgo test -target macos
```

可用的目标有 `macos`、`windows`、`linux`、`web`（在 Chrome、Chromium 或 Edge 中）、`ios`（在连接的 iPhone 上运行，连接多台时用 `-device` 指定，用 Xcode 上次使用的团队或 `-team` 指定的团队签名）、`ios_simulator`、`tvos_simulator`（方向键、作为选择键的 Return 和作为 Menu 键的 Escape 模拟遥控器）、`watchos_simulator` 和 `android`，或 `all`。每个测试在独立的进程中运行。`app.Capture()` 返回当前像素的 `image.Image`，`app.CapturePNG()` 返回 PNG 字节，无头运行时两者都会跳过测试。像素采用显示器的色彩空间，在配备 P3 显示器的 Mac 上是 Display P3：PNG 在色彩描述文件中标明了它，但 `image/png` 会忽略描述文件，所以从 `app.Capture()` 读到的颜色是该色彩空间的值而不是 sRGB 值，NSGo 也不会转换。`app.Screenshot("en/library")` 会把截图保存为 `.nsgo/test/screenshots/<target>/en/library.png`，在 `macos` 上即 `.nsgo/test/screenshots/macos/en/library.png`；名称可以包含空格，但不能是绝对路径或包含 `..`。`-screenshots dir` 改为保存到 `dir` 下，`dir` 是相对于当前工作目录的路径，路径与之前的截图重复会让该测试失败。`-fps` 会在 `macos`、`windows`、`linux` 和 `ios_simulator` 上每秒记录一次性能监视器的读数，在 `web` 和 `android` 上只记录引擎的读数。`-env KEY=VALUE` 为测试设置环境变量，手机上的测试只能这样拿到变量。在模拟器或手机上，没有 `nsgo.yaml` 的目录的测试会安装成以该目录命名的应用，ID 是 `dev.nsgo.` 加上目录名中的字母和数字，例如 `base/formula` 的是 `dev.nsgo.formula`。

Android 目标测试必须导入 `github.com/nsgo-dev/nsgo`（空导入即可），因为 Android 宿主使用测试进程的标准输出传输协议。对于没有该导入的库包，`nsgo test -target android` 会在构建或启动 APK 前直接拒绝；请对该包运行无头测试，或添加导入。

要测量而不是检查时，用 `nsgotest.Measure()` 启动：应用运行在系统时钟上，`Advance` 会真实等待相应时长，各步骤只等待正在进行的动画停下。`app.Mark("name")` 标出此后的读数测的是什么，`app.Mark("-")` 结束标记；无论是否加 `-fps`，标记都会打印出来。

## TSX 测试

TSX 应用中用 `nsgo/test` 编写的 `*.test.tsx` 文件会随 Go 测试一起运行。

不写测试文件也能驱动 TSX 应用：传入它的入口，并从标准输入提供步骤，写法与 `*.test.tsx` 测试的函数体相同，`app` 即该应用：

```sh
go tool nsgo test App.tsx < steps.ts
```
