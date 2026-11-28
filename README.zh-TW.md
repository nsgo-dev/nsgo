<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo：每個平台的原生 UI，以 Go 撰寫" width="100%">
  </picture>
</h1>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">简体中文</a> |
  <strong>繁體中文</strong> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <a href="README.fr.md">Français</a> |
  <a href="README.de.md">Deutsch</a> |
  <a href="README.pt.md">Português</a> |
  <a href="README.ru.md">Русский</a> |
  <a href="README.it.md">Italiano</a> |
  <a href="README.vi.md">Tiếng Việt</a> |
  <a href="README.id.md">Bahasa Indonesia</a> |
  <a href="README.tr.md">Türkçe</a>
</p>

<p align="center">
  <a href="https://github.com/nsgo-dev/nsgo/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/nsgo-dev/nsgo"></a>
  <a href="https://pkg.go.dev/github.com/nsgo-dev/nsgo"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/nsgo-dev/nsgo.svg"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
</p>

<p align="center">
  <a href="https://github.com/oneo-me"><img alt="GitHub" src="https://img.shields.io/badge/GitHub-oneo--me-181717?logo=github&logoColor=white"></a>
  <a href="https://oneo.me"><img alt="Blog" src="https://img.shields.io/badge/Blog-oneo.me-0969da"></a>
  <a href="https://x.com/oneo666"><img alt="X" src="https://img.shields.io/badge/X-%40oneo666-000000?logo=x&logoColor=white"></a>
  <a href="https://t.me/nsgo_dev"><img alt="Telegram" src="https://img.shields.io/badge/Telegram-%40nsgo__dev-26a5e4?logo=telegram&logoColor=white"></a>
  <a href="https://github.com/sponsors/oneo-me"><img alt="GitHub Sponsors" src="https://img.shields.io/badge/GitHub%20Sponsors-ea4aaa?logo=githubsponsors&logoColor=white"></a>
</p>

<p align="center">
  以 Go 為後端的跨平台原生 UI 引擎：應用程式邏輯、響應式圖與版面配置在 Go 中執行，各平台以自己的原生 UI 繪製。<br>
  它沿用 <a href="https://nsh.dev">Native Shell（NSH）</a> 的設計，適合想要 NSH 的模型、但不想寫 Rust 的團隊。
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="macOS 上 gallery 的 Stocks 範例" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="手機上 gallery 的元件清單" width="21.6%">
  </picture>
</p>

## 特色

- **原生視圖。** 文字、輸入法、捲動、選單與無障礙功能都採用各平台自己的實作；某個平台獨有的效果（例如 Liquid Glass），其他平台會加以模仿。
- **一台 Mac 建置所有平台。** Go 建置所有目標平台，iOS、tvOS 與 watchOS 另需 Xcode，只有 x86_64 Android 模擬器需要 NDK：不需要 Android SDK、JDK、GTK 或 Windows 工具鏈。
- **細粒度更新。** 元件只執行一次，訊號（signal）只更新讀取它的部分；支援 flexbox 與 grid 版面配置、類 CSS 樣式、Tailwind 類別、主題，動畫則跟隨平台的影格時脈。
- **元件。** `base` 提供無樣式元件，從按鈕到百萬列的 `DataTable`、圖表、流程圖、程式碼編輯器、富文字編輯器與終端機；`ui` 則為它們加上 NSGo 的外觀。
- **平台服務。** 導覽、通知、權限、選擇器、分享、觸覺回饋、App 內購買、小工具與即時動態；桌面上還有視窗、面板、選單、系統匣圖示與全域快速鍵。
- **在地化。** YAML 訊息目錄，支援 CLDR 複數規則、數字與日期格式，以及由右至左的版面配置。
- **TSX 應用程式。** 像 NSH 一樣以 TSX 撰寫應用程式，熱更新會保留狀態，並打包成由 Go 應用程式執行、經過簽署且沙盒化的套件。
- **像使用者一樣操作的測試。** `nsgotest` 會點按、輸入並讀取畫面上的內容，可無介面執行，也可在各平台上以原生輸入執行並截圖。
- **`nsgo` 工具。** 建立應用程式，以熱重載與 DevTools 執行，並打包發布到應用程式商店、Flathub，或提供含自動更新的直接下載。
- **小而快。** 一個計數器應用程式在桌面上打包後約 8 MB，在 NSGo 的效能測試中啟動速度勝過 NSH、MyGo 與 GPUI。

## 平台

| 平台 | 繪製方式 | 狀態 |
| --- | --- | --- |
| macOS | AppKit 與 Core Text | 可用 |
| iOS 及輕 App（App Clip） | UIKit 與 Core Text | 可在模擬器與實機上執行；輕 App 可在模擬器上執行 |
| tvOS | UIKit，由遙控器移動焦點 | 可在模擬器上執行 |
| watchOS | SwiftUI 與 Core Graphics | 可在模擬器上執行 |
| Android | Android View | 可在 arm64 實機與 x86_64 模擬器上執行 |
| Windows | Direct2D 與 DirectWrite | 可用 |
| Linux | GTK 4、Pango 與 Cairo | 可用 |
| Web | 由 WebAssembly 驅動的 DOM 元素 | 可用 |
| 鴻蒙 | ArkUI | 預覽 |
| 微信、QQ、支付寶與抖音小程式 | 各平台自己的視圖 | 實驗性，受小程式 2 MB 的大小限制 |

應用程式也可以嵌入既有的 iOS、Android、Go 桌面與 Web 應用程式。各目標平台如何對接系統，請參閱 [ARCHITECTURE.md](ARCHITECTURE.md#platforms)。

## 快速開始

安裝 [Go](https://go.dev/dl/) 1.27.3 或更新版本（建置 iOS、tvOS 與 watchOS 還需要 Xcode）與 `nsgo`（[其他安裝方式](docs/public/en/getting-started.md#install-nsgo)）後，建立應用程式並以熱重載執行：

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # 加上 -template tsx 建立 TSX 應用程式
cd hello
nsgo dev    # 加上 -target web、ios_simulator、tvos_simulator、watchos_simulator、android 或 miniprogram_wechat 在其他平台執行
nsgo doctor # 列出本機能建置、執行哪些目標平台，以及其餘平台缺少什麼
```

應用程式由回傳元素的 Go 元件組成，訊號變化時只更新讀取它的部分：

```go
package main

import (
	"fmt"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{Style: core.Style{Padding: core.All(24), Gap: 12}, Children: []core.Node{
		core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }},
		ui.Button{Text: "Press me", OnPress: func() { count.Update(func(n int) int { return n + 1 }) }},
	}}
}

func main() { nsgo.Run(Root) }
```

測試像使用者一樣操作應用程式，`nsgo test` 可無介面執行它們，加上 `-target` 則在指定平台上執行：

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

想在發布前試用 `main` 上的最新程式碼，請執行 `go get github.com/nsgo-dev/nsgo@main`。

## 狀態

NSGo 仍處於早期階段，API 尚未穩定。各版本的變更請見 [Releases](https://github.com/nsgo-dev/nsgo/releases)，尚缺少的部分請見[路線圖](docs/roadmap.md)。

## 文件

[文件](https://nsgo.dev/zh-tw/docs/)介紹元件與狀態、版面配置、樣式、導覽、元件參考、TSX 應用程式、在地化、測試、嵌入，以及建置與發布；[線上展示](https://nsgo.dev/gallery/)可在瀏覽器中執行 gallery。

程式設計代理可以透過 NSGo 的技能學習它（`bunx skills add nsgo-dev/nsgo`），並從 [llms.txt](https://nsgo.dev/llms.txt) 以 Markdown 閱讀文件。

## 開發

```sh
go tool nsgo dev examples/gallery   # 執行 gallery
go tool nsgo check                  # 檢查所有套件
go tool nsgo test                   # 執行所有套件的測試
```

其餘的開發環境設定請見[開發環境設定](https://nsgo.dev/zh-tw/docs/contributing/)。修改程式碼前請先閱讀 [ARCHITECTURE.md](ARCHITECTURE.md) 與 [AGENTS.md](AGENTS.md)，上次發布以來的變更請見 [CHANGELOG.md](CHANGELOG.md)。

## 社群

歡迎到 Telegram 群組 nsgo-dev-community（[@nsgo_dev](https://t.me/nsgo_dev)）提問、交流想法，或分享你用 NSGo 打造的作品。Bug 與修改建議請提交到 [issues](https://github.com/nsgo-dev/nsgo/issues)。

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="Telegram 群組 @nsgo_dev 的 QR 碼" width="200"></a>
</p>

## 贊助

[Depot](https://depot.dev) 贊助 NSGo 的 CI：它的 runner 在 Linux、macOS 與 Windows 上建置並測試 NSGo。

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## 授權

本專案採用 [Apache License 2.0](LICENSE) 授權。NSGo 是 NSH 的移植版本，[NOTICE](NOTICE) 記載了移植所需的署名。

除非你另有明確聲明，否則你有意提交以納入 NSGo 的任何貢獻，依 Apache-2.0 授權條款的定義，皆以 Apache-2.0 授權，不附加任何額外條款或條件。
