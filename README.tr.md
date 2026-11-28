<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: Go ile yazılmış her platformda yerel arayüz" width="100%">
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
  <a href="README.id.md">Bahasa Indonesia</a> |
  <strong>Türkçe</strong>
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
  Go arka uçlu çapraz platform yerel arayüz motoru: uygulama mantığı, tepkisel (reactive) grafik ve yerleşim Go içinde çalışır; her platform kendi yerel arayüzüyle çizer.<br>
  Rust yazmadan NSH'nin modelini isteyen ekipler için <a href="https://nsh.dev">Native Shell (NSH)</a> tasarımını izler.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="macOS üzerinde galerinin Stocks demosu" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="Telefonda galerinin bileşen listesi" width="21.6%">
  </picture>
</p>

## Öne çıkanlar

- **Yerel görünümler.** Metin, giriş yöntemleri, kaydırma, menüler ve erişilebilirlik her platformun kendi bileşenleridir; bir platformda olan (örneğin Liquid Glass) özellikler diğerlerinde taklit edilir.
- **Tek bir Mac'ten tüm platformlar.** Go tüm hedefleri derler; yalnızca iOS, tvOS ve watchOS için Xcode ve x86_64 Android emülatörü için NDK gerekir: Android SDK, JDK, GTK veya Windows araç zinciri gerekmez.
- **İnce taneli güncellemeler.** Bileşenler bir kez çalışır ve sinyaller yalnızca kendilerini okuyan kısımları günceller; flexbox ve grid yerleşimi, CSS benzeri stiller, Tailwind sınıfları, temalar ve platformun kare saatine bağlı animasyonlarla birlikte.
- **Bileşenler.** `base` düğmelerden milyon satırlı `DataTable` bileşenine, grafiklere, akış diyagramlarına, kod düzenleyiciye, zengin metin düzenleyiciye ve terminale kadar stilsiz bileşenler sunar; `ui` ise bunlara NSGo görünümünü verir.
- **Platform servisleri.** Gezinme, bildirimler, izinler, seçiciler, paylaşım, dokunsal geri bildirim, uygulama içi satın alımlar, araç takımları ve Canlı Etkinlikler; masaüstünde pencereler, paneller, menüler, sistem tepsisi simgeleri ve genel kısayollar.
- **Yerelleştirme.** CLDR çoğulları, sayılar, tarihler ve sağdan sola (RTL) yerleşimle YAML mesaj katalogları.
- **TSX uygulamaları.** NSH'de olduğu gibi durumu koruyan sıcak güncellemelerle TSX uygulamaları yazın ve bunları bir Go uygulamasının çalıştırdığı imzalı, korumalı paketler olarak dağıtın.
- **Kullanıcı gibi davranan testler.** `nsgotest` ekrandakine basar, yazar ve okur — başsız olarak veya her platformda yerel girdi ve ekran görüntüleriyle.
- **`nsgo` aracı.** Uygulamalar oluşturun, sıcak yeniden yükleme ve DevTools ile çalıştırın ve uygulama mağazaları, Flathub ve güncellemeli doğrudan indirmeler için paketleyin.
- **Küçük ve hızlı.** Bir sayaç uygulaması masaüstünde yaklaşık 8 MB tutar ve NSGo kıyaslamalarında NSH, MyGo ve GPUI'den daha hızlı başlar.

## Platformlar

| Platform | Çizim motoru | Durum |
| --- | --- | --- |
| macOS | AppKit ve Core Text | Çalışıyor |
| iOS ve App Clips | UIKit ve Core Text | Simülatörde ve cihazlarda çalışıyor; App Clips simülatörde |
| tvOS | Kumanda odağıyla yönlendirilen UIKit | Simülatörde çalışıyor |
| watchOS | SwiftUI ve Core Graphics | Simülatörde çalışıyor |
| Android | Android Views | arm64 cihazlarda ve x86_64 emülatörlerde çalışıyor |
| Windows | Direct2D ve DirectWrite | Çalışıyor |
| Linux | GTK 4, Pango ve Cairo | Çalışıyor |
| Web | WebAssembly üzerinden DOM öğeleri | Çalışıyor |
| HarmonyOS | ArkUI | Önizleme |
| WeChat, QQ, Alipay ve Douyin mini programları | Her platformun kendi görünümleri | Deneysel, 2 MB sınırları içinde |

Uygulamalar ayrıca mevcut iOS, Android, Go masaüstü ve web uygulamalarının içine de gömülebilir. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) her hedefin sistemine nasıl ulaştığını açıklar.

## Başlarken

[Go](https://go.dev/dl/) 1.27.3 veya üzeri (ve iOS, tvOS, watchOS için Xcode) ile `nsgo` aracını kurun ([diğer yöntemler](docs/public/en/getting-started.md#install-nsgo)), ardından bir uygulama oluşturup sıcak yeniden yükleme ile çalıştırın:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # TSX uygulaması için -template tsx
cd hello
nsgo dev    # başka yerde çalıştırmak için -target web, ios_simulator, tvos_simulator, watchos_simulator, android veya miniprogram_wechat
nsgo doctor # bu makinenin hangi hedefleri derleyip çalıştırdığını ve diğerlerinin neye ihtiyacı olduğunu gösterir
```

Bir uygulama, kendilerini okuyan kısımları sinyallerle güncelleyen ve öğeler döndüren Go bileşenlerinden oluşur:

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

Testleri uygulamayı tıpkı bir kullanıcı gibi kullanır ve `nsgo test` bunları başsız veya `-target` ile bir platform üzerinde çalıştırır:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Yayımlanmadan önce `main` üzerindeki yenilikleri denemek için `go get github.com/nsgo-dev/nsgo@main` çalıştırın.

## Durum

NSGo henüz erken aşamadadır ve API'si henüz kararlı değildir. [Releases](https://github.com/nsgo-dev/nsgo/releases) her sürümde nelerin değiştiğini, [yol haritası](docs/roadmap.md) ise hâlâ nelerin eksik olduğunu listeler.

## Belgeler

[Belgeler](https://nsgo.dev/tr/docs/) bileşenler ve durum, yerleşim, stiller, gezinme, bileşen referansı, TSX uygulamaları, yerelleştirme, test, gömme, derleme ve dağıtımı kapsar; [canlı demo](https://nsgo.dev/gallery/) galeriyi tarayıcıda çalıştırır.

Kodlama ajanları NSGo'yu `bunx skills add nsgo-dev/nsgo` becerisinden öğrenir ve belgeleri [llms.txt](https://nsgo.dev/llms.txt) üzerinden Markdown olarak okur.

## Geliştirme

```sh
go tool nsgo dev examples/gallery   # galeriyi çalıştır
go tool nsgo check                  # tüm paketleri denetle
go tool nsgo test                   # tüm paketlerin testlerini çalıştır
```

Kurulumun geri kalanı [Katkıda Bulunma](https://nsgo.dev/tr/docs/contributing/) sayfasındadır. Kod değiştirmeden önce [ARCHITECTURE.md](ARCHITECTURE.md) ve [AGENTS.md](AGENTS.md) dosyalarını, son sürümden beri değişenler için [CHANGELOG.md](CHANGELOG.md) dosyasını okuyun.

## Topluluk

Sorularınız, fikirleriniz ve NSGo ile geliştirdikleriniz için `nsgo-dev-community` Telegram grubuna [@nsgo_dev](https://t.me/nsgo_dev) katılabilirsiniz. Hataları bildirmek ve değişiklik önermek için [issues](https://github.com/nsgo-dev/nsgo/issues) kullanın.

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="@nsgo_dev Telegram grubu QR kodu" width="200"></a>
</p>

## Sponsorlar

[Depot](https://depot.dev), NSGo'nun CI altyapısına sponsor olmaktadır: çalıştırıcıları NSGo'yu Linux, macOS ve Windows üzerinde derler ve test eder.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Lisans

[Apache License, Version 2.0](LICENSE) altında lisanslanmıştır. NSGo, NSH'nin bir portudur ve [NOTICE](NOTICE) bu portun atıf kayıtlarını içerir.

Aksi açıkça belirtilmedikçe, Apache-2.0 lisansında tanımlandığı şekilde NSGo'ya dahil edilmek üzere tarafınızdan bilerek gönderilen her katkı, ek şart veya koşul olmaksızın Apache-2.0 kapsamında lisanslanır.
