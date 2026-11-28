<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: UI native di setiap platform, ditulis dengan Go" width="100%">
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
  <a href="README.vi.md">Tiếng Việt</a> |
  <strong>Bahasa Indonesia</strong> |
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
  Mesin UI native lintas platform dengan backend Go: logika aplikasi, graf reaktif, dan tata letak berjalan di Go, sementara setiap platform menggambar dengan UI native-nya sendiri.<br>
  NSGo mengikuti desain <a href="https://nsh.dev">Native Shell (NSH)</a>, untuk tim yang menginginkan model NSH tanpa menulis Rust.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="Demo Stocks dari gallery di macOS" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="Daftar komponen gallery di ponsel" width="21.6%">
  </picture>
</p>

## Sorotan

- **View native.** Teks, metode input, gulir, menu, dan aksesibilitas adalah milik masing-masing platform, dan apa yang dimiliki satu platform, seperti Liquid Glass, ditiru oleh platform lainnya.
- **Semua platform dari satu Mac.** Go membangun setiap target, dengan Xcode untuk iOS, tvOS, dan watchOS serta NDK hanya untuk emulator Android x86_64: tanpa Android SDK, JDK, GTK, atau toolchain Windows.
- **Pembaruan yang presisi.** Komponen berjalan sekali dan signal hanya memperbarui bagian yang membacanya, dengan tata letak flexbox dan grid, gaya mirip CSS, kelas Tailwind, tema, dan animasi pada frame clock platform.
- **Komponen.** `base` menyediakan komponen tanpa gaya, dari tombol hingga `DataTable` sejuta baris, grafik, diagram alir, editor kode, editor teks kaya, dan terminal, sementara `ui` memberinya tampilan NSGo.
- **Layanan platform.** Navigasi, notifikasi, izin, pemilih, berbagi, haptik, pembelian dalam aplikasi, widget, dan Live Activities, serta di desktop: jendela, panel, menu, ikon tray, dan pintasan global.
- **Terlokalisasi.** Katalog pesan YAML dengan bentuk jamak, angka, dan tanggal CLDR, serta tata letak kanan-ke-kiri.
- **Aplikasi TSX.** Tulis aplikasi dengan TSX seperti NSH, dengan pembaruan langsung yang mempertahankan state, lalu kirimkan sebagai paket bertanda tangan dan ber-sandbox yang dijalankan oleh aplikasi Go.
- **Pengujian yang bertindak seperti pengguna.** `nsgotest` menekan, mengetik, dan membaca apa yang ada di layar, secara headless atau di setiap platform dengan input native dan tangkapan layar.
- **Tool `nsgo`.** Buat aplikasi, jalankan dengan hot reload dan DevTools, lalu kemas untuk toko aplikasi, Flathub, dan unduhan langsung dengan pembaruan.
- **Kecil dan cepat.** Aplikasi penghitung berukuran sekitar 8 MB di desktop dan mulai lebih cepat daripada NSH, MyGo, dan GPUI dalam tolok ukur NSGo.

## Platform

| Platform | Menggambar dengan | Status |
| --- | --- | --- |
| macOS | AppKit dan Core Text | Berjalan |
| iOS dan App Clips | UIKit dan Core Text | Berjalan di simulator dan perangkat; App Clips di simulator |
| tvOS | UIKit, dikendalikan oleh fokus remote | Berjalan di simulator |
| watchOS | SwiftUI dan Core Graphics | Berjalan di simulator |
| Android | Android Views | Berjalan di perangkat arm64 dan emulator x86_64 |
| Windows | Direct2D dan DirectWrite | Berjalan |
| Linux | GTK 4, Pango, dan Cairo | Berjalan |
| Web | Elemen DOM, dari WebAssembly | Berjalan |
| HarmonyOS | ArkUI | Pratinjau |
| Program mini WeChat, QQ, Alipay, dan Douyin | View masing-masing platform | Eksperimental, dalam batas 2 MB mereka |

Aplikasi juga dapat disematkan ke dalam aplikasi iOS, Android, desktop Go, dan web yang sudah ada. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) menjelaskan bagaimana setiap target menjangkau sistemnya.

## Mulai

Dengan [Go](https://go.dev/dl/) 1.27.3 atau lebih baru, serta Xcode untuk iOS, tvOS, dan watchOS, pasang `nsgo` ([cara lain](docs/public/en/getting-started.md#install-nsgo)), lalu buat aplikasi dan jalankan dengan hot reload:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # -template tsx untuk aplikasi TSX
cd hello
nsgo dev    # -target web, ios_simulator, tvos_simulator, watchos_simulator, android, atau miniprogram_wechat untuk menjalankan di tempat lain
nsgo doctor # target mana yang dapat dibangun dan dijalankan mesin ini, dan apa yang kurang untuk yang lain
```

Aplikasi adalah komponen Go yang mengembalikan elemen, dengan signal yang memperbarui bagian yang membacanya:

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

Pengujiannya menggunakan aplikasi seperti pengguna, dan `nsgo test` menjalankannya secara headless atau, dengan `-target`, di sebuah platform:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Untuk mencoba apa yang ada di `main` sebelum dirilis, jalankan `go get github.com/nsgo-dev/nsgo@main`.

## Status

NSGo masih tahap awal, dan API-nya belum stabil. [Rilis](https://github.com/nsgo-dev/nsgo/releases) mencantumkan perubahan di setiap rilis, dan [peta jalan](docs/roadmap.md) mencantumkan apa yang masih kurang.

## Dokumentasi

[Dokumentasi](https://nsgo.dev/id/docs/) mencakup komponen dan state, tata letak, gaya, navigasi, referensi komponen, aplikasi TSX, lokalisasi, pengujian, penyematan, build, dan distribusi; [demo langsung](https://nsgo.dev/gallery/) menjalankan gallery di browser.

Agen pemrograman mempelajari NSGo dari skill-nya, `bunx skills add nsgo-dev/nsgo`, dan membaca dokumentasi dalam format Markdown dari [llms.txt](https://nsgo.dev/llms.txt).

## Pengembangan

```sh
go tool nsgo dev examples/gallery   # menjalankan gallery
go tool nsgo check                  # memeriksa setiap paket
go tool nsgo test                   # pengujian setiap paket
```

[Pengaturan pengembangan](https://nsgo.dev/id/docs/contributing/) berisi sisa penyiapannya. Baca [ARCHITECTURE.md](ARCHITECTURE.md) dan [AGENTS.md](AGENTS.md) sebelum mengubah kode, dan [CHANGELOG.md](CHANGELOG.md) untuk apa yang berubah sejak rilis terakhir.

## Komunitas

Pertanyaan, ide, dan apa yang Anda bangun dengan NSGo disambut di nsgo-dev-community, grup Telegram [@nsgo_dev](https://t.me/nsgo_dev). Laporkan bug dan usulkan perubahan di [issue](https://github.com/nsgo-dev/nsgo/issues).

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="Kode QR grup Telegram @nsgo_dev" width="200"></a>
</p>

## Sponsor

[Depot](https://depot.dev) mensponsori CI NSGo: runner-nya membangun dan menguji NSGo di Linux, macOS, dan Windows.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Lisensi

Dilisensikan di bawah [Apache License, Version 2.0](LICENSE). NSGo adalah port dari NSH, dan [NOTICE](NOTICE) mencatat atribusi yang menjadi kewajiban port tersebut.

Kecuali Anda menyatakan sebaliknya secara eksplisit, setiap kontribusi yang Anda kirimkan dengan sengaja untuk disertakan dalam NSGo, sebagaimana didefinisikan dalam lisensi Apache-2.0, akan dilisensikan di bawah Apache-2.0, tanpa syarat atau ketentuan tambahan apa pun.
