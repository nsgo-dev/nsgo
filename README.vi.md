<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: giao diện gốc trên mọi nền tảng, viết bằng Go" width="100%">
  </picture>
</h1>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">简体中文</a> |
  <a href="README.zh-TW.md">繁體中文</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <a href="README.fr.md">Français</a> |
  <a href="README.de.md">Deutsch</a> |
  <a href="README.pt.md">Português</a> |
  <a href="README.ru.md">Русский</a> |
  <a href="README.it.md">Italiano</a> |
  <strong>Tiếng Việt</strong> |
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
  Engine giao diện gốc đa nền tảng với backend Go: logic ứng dụng, đồ thị phản ứng (reactive graph) và bố cục chạy trong Go, còn mỗi nền tảng vẽ bằng giao diện gốc của riêng mình.<br>
  NSGo theo thiết kế của <a href="https://nsh.dev">Native Shell (NSH)</a>, dành cho các nhóm muốn có mô hình của NSH mà không phải viết Rust.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="Bản demo Stocks của gallery trên macOS" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="Danh sách component của gallery trên điện thoại" width="21.6%">
  </picture>
</p>

## Điểm nổi bật

- **View gốc.** Văn bản, bộ gõ, cuộn, menu và trợ năng đều là của chính từng nền tảng, và những gì một nền tảng có, như Liquid Glass, các nền tảng khác sẽ mô phỏng theo.
- **Mọi nền tảng từ một máy Mac.** Go build mọi nền tảng đích, cùng Xcode cho iOS, tvOS và watchOS, và NDK chỉ cho trình giả lập Android x86_64: không cần Android SDK, JDK, GTK hay toolchain Windows.
- **Cập nhật chi tiết.** Component chỉ chạy một lần và signal chỉ cập nhật những gì đọc nó, với bố cục flexbox và grid, style giống CSS, class Tailwind, theme và animation theo frame clock của nền tảng.
- **Component.** `base` cung cấp các component không style, từ nút bấm đến `DataTable` một triệu dòng, biểu đồ, sơ đồ luồng, trình soạn thảo mã, trình soạn thảo văn bản định dạng và terminal, còn `ui` mang lại cho chúng giao diện của NSGo.
- **Dịch vụ nền tảng.** Điều hướng, thông báo, quyền, bộ chọn, chia sẻ, phản hồi xúc giác, mua hàng trong ứng dụng, widget và Live Activities, cùng trên desktop là cửa sổ, panel, menu, biểu tượng khay hệ thống và phím tắt toàn cục.
- **Bản địa hóa.** Danh mục thông điệp YAML với số nhiều, số và ngày tháng theo CLDR, cùng bố cục từ phải sang trái.
- **Ứng dụng TSX.** Viết ứng dụng bằng TSX như NSH, với cập nhật nóng giữ nguyên trạng thái, và phát hành chúng dưới dạng gói đã ký, chạy trong sandbox để một ứng dụng Go chạy.
- **Kiểm thử hành động như người dùng.** `nsgotest` nhấn, gõ và đọc những gì trên màn hình, ở chế độ headless hoặc trên từng nền tảng với đầu vào gốc và ảnh chụp màn hình.
- **Công cụ `nsgo`.** Tạo ứng dụng, chạy với hot reload và DevTools, rồi đóng gói cho các kho ứng dụng, Flathub và tải trực tiếp kèm cập nhật.
- **Nhỏ gọn và nhanh.** Một ứng dụng đếm số chỉ khoảng 8 MB trên desktop và khởi động nhanh hơn NSH, MyGo và GPUI trong các benchmark của NSGo.

## Nền tảng

| Nền tảng | Vẽ bằng | Trạng thái |
| --- | --- | --- |
| macOS | AppKit và Core Text | Chạy được |
| iOS và App Clips | UIKit và Core Text | Chạy trên trình giả lập và thiết bị; App Clips trên trình giả lập |
| tvOS | UIKit, điều khiển bằng focus của remote | Chạy trên trình giả lập |
| watchOS | SwiftUI và Core Graphics | Chạy trên trình giả lập |
| Android | Android Views | Chạy trên thiết bị arm64 và trình giả lập x86_64 |
| Windows | Direct2D và DirectWrite | Chạy được |
| Linux | GTK 4, Pango và Cairo | Chạy được |
| Web | Phần tử DOM, từ WebAssembly | Chạy được |
| HarmonyOS | ArkUI | Xem trước |
| Mini program WeChat, QQ, Alipay và Douyin | View của từng nền tảng | Thử nghiệm, trong giới hạn 2 MB của chúng |

Ứng dụng cũng có thể nhúng vào các ứng dụng iOS, Android, desktop Go và web có sẵn. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) mô tả cách mỗi nền tảng đích tiếp cận hệ thống của nó.

## Bắt đầu

Với [Go](https://go.dev/dl/) 1.27.3 trở lên, cùng Xcode cho iOS, tvOS và watchOS, hãy cài `nsgo` ([cách khác](docs/public/en/getting-started.md#install-nsgo)), rồi tạo một ứng dụng và chạy nó với hot reload:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # thêm -template tsx để tạo ứng dụng TSX
cd hello
nsgo dev    # thêm -target web, ios_simulator, tvos_simulator, watchos_simulator, android hoặc miniprogram_wechat để chạy ở nơi khác
nsgo doctor # máy này build và chạy được những nền tảng đích nào, và các nền tảng còn lại thiếu gì
```

Một ứng dụng là các component Go trả về element, với signal cập nhật những gì đọc nó:

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

Các kiểm thử của nó dùng ứng dụng như người dùng, và `nsgo test` chạy chúng ở chế độ headless hoặc, với `-target`, trên một nền tảng:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Để thử những gì đang có trên `main` trước khi phát hành, hãy chạy `go get github.com/nsgo-dev/nsgo@main`.

## Trạng thái

NSGo còn ở giai đoạn đầu và API chưa ổn định. [Releases](https://github.com/nsgo-dev/nsgo/releases) liệt kê những thay đổi của mỗi bản phát hành, còn [lộ trình](docs/roadmap.md) liệt kê những gì vẫn còn thiếu.

## Tài liệu

[Tài liệu](https://nsgo.dev/vi/docs/) bao gồm component và trạng thái, bố cục, style, điều hướng, tài liệu tham khảo component, ứng dụng TSX, bản địa hóa, kiểm thử, nhúng, build và phát hành; [bản demo trực tuyến](https://nsgo.dev/gallery/) chạy gallery ngay trong trình duyệt.

Các coding agent học NSGo từ skill của nó, `bunx skills add nsgo-dev/nsgo`, và đọc tài liệu dạng Markdown từ [llms.txt](https://nsgo.dev/llms.txt).

## Phát triển

```sh
go tool nsgo dev examples/gallery   # chạy gallery
go tool nsgo check                  # kiểm tra mọi package
go tool nsgo test                   # kiểm thử của mọi package
```

[Thiết lập môi trường phát triển](https://nsgo.dev/vi/docs/contributing/) có phần thiết lập còn lại. Hãy đọc [ARCHITECTURE.md](ARCHITECTURE.md) và [AGENTS.md](AGENTS.md) trước khi sửa mã, và [CHANGELOG.md](CHANGELOG.md) để biết những gì đã thay đổi kể từ bản phát hành gần nhất.

## Cộng đồng

Câu hỏi, ý tưởng và những gì bạn xây dựng với NSGo đều được chào đón tại nsgo-dev-community, nhóm Telegram [@nsgo_dev](https://t.me/nsgo_dev). Hãy báo lỗi và đề xuất thay đổi trong [issues](https://github.com/nsgo-dev/nsgo/issues).

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="Mã QR của nhóm Telegram @nsgo_dev" width="200"></a>
</p>

## Nhà tài trợ

[Depot](https://depot.dev) tài trợ CI cho NSGo: các runner của Depot build và kiểm thử NSGo trên Linux, macOS và Windows.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Giấy phép

Phát hành theo [Apache License, Version 2.0](LICENSE). NSGo là bản port của NSH, và [NOTICE](NOTICE) ghi nhận phần ghi công mà bản port này cần có.

Trừ khi bạn nêu rõ khác đi, mọi đóng góp bạn chủ ý gửi để đưa vào NSGo, theo định nghĩa của giấy phép Apache-2.0, sẽ được cấp phép theo Apache-2.0, không kèm bất kỳ điều khoản hay điều kiện bổ sung nào.
