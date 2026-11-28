<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo：あらゆるプラットフォームのネイティブ UI を Go で" width="100%">
  </picture>
</h1>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">简体中文</a> |
  <a href="README.zh-TW.md">繁體中文</a> |
  <strong>日本語</strong> |
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
  Go をバックエンドとするクロスプラットフォームのネイティブ UI エンジンです。アプリのロジック、リアクティブグラフ、レイアウトは Go で動き、描画は各プラットフォームのネイティブ UI が行います。<br>
  <a href="https://nsh.dev">Native Shell（NSH）</a>の設計を踏襲しており、Rust を書かずに NSH のモデルを使いたいチームに向けたものです。
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="macOS で動くギャラリーの Stocks デモ" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="スマートフォンで表示したギャラリーのコンポーネント一覧" width="21.6%">
  </picture>
</p>

## 特長

- **ネイティブビュー。** テキスト、入力メソッド、スクロール、メニュー、アクセシビリティはすべて各プラットフォーム本来のものを使い、Liquid Glass のようにあるプラットフォームにしかないものは、ほかのプラットフォームで再現します。
- **1 台の Mac ですべてのプラットフォームを。** すべてのターゲットを Go でビルドでき、必要なのは iOS、tvOS、watchOS 向けの Xcode と、x86_64 Android エミュレータ向けの NDK だけです。Android SDK、JDK、GTK、Windows ツールチェーンは要りません。
- **きめ細かな更新。** コンポーネントは一度だけ実行され、シグナルはそれを読む部分だけを更新します。flexbox と grid のレイアウト、CSS 風のスタイル、Tailwind のクラス、テーマ、プラットフォームのフレームクロックに合わせたアニメーションを備えます。
- **コンポーネント。** `base` はボタンから 100 万行の `DataTable`、チャート、フロー図、コードエディタ、リッチテキストエディタ、ターミナルまで、スタイルなしのコンポーネントを提供し、`ui` がそれらに NSGo の見た目を与えます。
- **プラットフォームサービス。** ナビゲーション、通知、権限、ピッカー、共有、触覚フィードバック、App 内課金、ウィジェット、ライブアクティビティ。デスクトップではさらにウインドウ、パネル、メニュー、トレイアイコン、グローバルショートカットも使えます。
- **ローカライズ。** CLDR の複数形・数値・日付に対応した YAML のメッセージカタログと、右から左へのレイアウト。
- **TSX アプリ。** NSH と同じように TSX でアプリを書け、状態を保ったままホットアップデートでき、Go アプリが実行する署名済み・サンドボックス化されたパッケージとして配布できます。
- **ユーザーと同じように操作するテスト。** `nsgotest` は画面上の要素を押し、入力し、読み取ります。ヘッドレスでも、各プラットフォーム上でネイティブ入力とスクリーンショットを使っても実行できます。
- **`nsgo` ツール。** アプリを作成し、ホットリロードと DevTools 付きで実行し、アプリストア、Flathub、アップデート付きの直接ダウンロード向けにパッケージングします。
- **小さく速い。** カウンターアプリはデスクトップでおよそ 8 MB に収まり、NSGo のベンチマークでは NSH、MyGo、GPUI より速く起動します。

## プラットフォーム

| プラットフォーム | 描画 | 状況 |
| --- | --- | --- |
| macOS | AppKit と Core Text | 動作 |
| iOS と App Clips | UIKit と Core Text | シミュレータと実機で動作。App Clips はシミュレータで動作 |
| tvOS | UIKit（リモコンのフォーカスで操作） | シミュレータで動作 |
| watchOS | SwiftUI と Core Graphics | シミュレータで動作 |
| Android | Android View | arm64 実機と x86_64 エミュレータで動作 |
| Windows | Direct2D と DirectWrite | 動作 |
| Linux | GTK 4、Pango、Cairo | 動作 |
| Web | WebAssembly から操作する DOM 要素 | 動作 |
| HarmonyOS | ArkUI | プレビュー |
| WeChat、QQ、Alipay、Douyin のミニプログラム | 各プラットフォームのビュー | 実験的。2 MB の制限内で動作 |

アプリは既存の iOS、Android、Go デスクトップ、Web アプリに組み込むこともできます。各ターゲットがどのようにシステムとつながるかは [ARCHITECTURE.md](ARCHITECTURE.md#platforms) を参照してください。

## はじめに

[Go](https://go.dev/dl/) 1.27.3 以降（iOS、tvOS、watchOS 向けには Xcode も）を用意して `nsgo` をインストールし（[ほかの方法](docs/public/en/getting-started.md#install-nsgo)）、アプリを作成してホットリロードで実行します：

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # TSX アプリなら -template tsx
cd hello
nsgo dev    # ほかで実行するなら -target web、ios_simulator、tvos_simulator、watchos_simulator、android、miniprogram_wechat
nsgo doctor # このマシンでビルド・実行できるターゲットと、ほかのターゲットに足りないもの
```

アプリは要素を返す Go のコンポーネントでできており、シグナルがそれを読む部分を更新します：

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

テストはユーザーと同じようにアプリを操作し、`nsgo test` はヘッドレスで、`-target` を付ければ指定したプラットフォーム上で実行します：

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

リリース前の `main` を試すには、`go get github.com/nsgo-dev/nsgo@main` を実行します。

## 現状

NSGo は開発の初期段階にあり、API はまだ安定していません。各リリースの変更点は [Releases](https://github.com/nsgo-dev/nsgo/releases) に、まだ足りないものは[ロードマップ](docs/roadmap.md)にまとめています。

## ドキュメント

[ドキュメント](https://nsgo.dev/ja/docs/)では、コンポーネントと状態、レイアウト、スタイル、ナビゲーション、コンポーネントリファレンス、TSX アプリ、ローカライズ、テスト、埋め込み、ビルドと配布を扱います。[ライブデモ](https://nsgo.dev/gallery/)ではギャラリーをブラウザで動かせます。

コーディングエージェントは NSGo のスキル（`bunx skills add nsgo-dev/nsgo`）で NSGo を学び、[llms.txt](https://nsgo.dev/llms.txt) からドキュメントを Markdown で読めます。

## 開発

```sh
go tool nsgo dev examples/gallery   # ギャラリーを実行
go tool nsgo check                  # すべてのパッケージを検査
go tool nsgo test                   # すべてのパッケージのテストを実行
```

ほかのセットアップは[開発セットアップ](https://nsgo.dev/ja/docs/contributing/)にあります。コードを変更する前に [ARCHITECTURE.md](ARCHITECTURE.md) と [AGENTS.md](AGENTS.md) を読み、前回のリリース以降の変更は [CHANGELOG.md](CHANGELOG.md) で確認してください。

## コミュニティ

質問やアイデア、NSGo で作ったものの共有は、Telegram グループ nsgo-dev-community（[@nsgo_dev](https://t.me/nsgo_dev)）で歓迎しています。バグの報告や変更の提案は [issues](https://github.com/nsgo-dev/nsgo/issues) へどうぞ。

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="Telegram グループ @nsgo_dev の QR コード" width="200"></a>
</p>

## スポンサー

[Depot](https://depot.dev) が NSGo の CI を支援しています。そのランナーが Linux、macOS、Windows で NSGo をビルド・テストしています。

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## ライセンス

[Apache License 2.0](LICENSE) のもとで提供しています。NSGo は NSH の移植であり、移植に伴う帰属表示は [NOTICE](NOTICE) に記載しています。

明示的に別段の定めをしない限り、あなたが NSGo に含めることを意図して提出したコントリビューションは、Apache-2.0 ライセンスの定義に従い、追加の条項や条件なしに Apache-2.0 のもとでライセンスされるものとします。
