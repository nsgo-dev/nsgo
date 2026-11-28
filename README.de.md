<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: native Oberflächen auf jeder Plattform, geschrieben in Go" width="100%">
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
  <strong>Deutsch</strong> |
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
  Eine plattformübergreifende native UI-Engine mit Go-Backend: App-Logik, reaktiver Graph und Layout laufen in Go, und jede Plattform zeichnet mit ihrer eigenen nativen UI.<br>
  Sie folgt dem Design von <a href="https://nsh.dev">Native Shell (NSH)</a> – für Teams, die das Modell von NSH wollen, ohne Rust zu schreiben.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="Die Stocks-Demo der Galerie unter macOS" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="Die Komponentenliste der Galerie auf einem Smartphone" width="21.6%">
  </picture>
</p>

## Highlights

- **Native Views.** Text, Eingabemethoden, Scrollen, Menüs und Barrierefreiheit stammen von der jeweiligen Plattform, und was eine Plattform hat, etwa Liquid Glass, ahmen die anderen nach.
- **Jede Plattform von einem Mac aus.** Go baut jede Zielplattform, mit Xcode für iOS, tvOS und watchOS und einem NDK nur für den x86_64-Android-Emulator: kein Android SDK, kein JDK, kein GTK und keine Windows-Toolchain.
- **Feingranulare Updates.** Komponenten laufen einmal, und Signale aktualisieren nur, was sie liest – mit Flexbox- und Grid-Layout, CSS-ähnlichen Styles, Tailwind-Klassen, Themes und Animationen im Frame-Takt der Plattform.
- **Komponenten.** `base` bietet ungestylte Komponenten von Buttons bis zu einer `DataTable` mit einer Million Zeilen, Diagramme, Flussdiagramme, einen Code-Editor, einen Rich-Text-Editor und ein Terminal, und `ui` gibt ihnen den Look von NSGo.
- **Plattformdienste.** Navigation, Benachrichtigungen, Berechtigungen, Auswahldialoge, Teilen, Haptik, In-App-Käufe, Widgets und Live Activities, auf dem Desktop außerdem Fenster, Panels, Menüs, Tray-Icons und globale Tastenkürzel.
- **Lokalisiert.** YAML-Nachrichtenkataloge mit CLDR-Pluralformen, -Zahlen und -Datumsangaben sowie Layout von rechts nach links.
- **TSX-Apps.** Schreiben Sie Apps in TSX wie mit NSH, mit Hot Updates, die den Zustand erhalten, und liefern Sie sie als signierte, gesandboxte Pakete aus, die eine Go-App ausführt.
- **Tests, die wie Nutzer handeln.** `nsgotest` drückt, tippt und liest, was auf dem Bildschirm steht – headless oder auf jeder Plattform mit nativer Eingabe und Screenshots.
- **Das Tool `nsgo`.** Erstellen Sie Apps, führen Sie sie mit Hot Reload und DevTools aus und paketieren Sie sie für App Stores, Flathub und direkte Downloads mit Updates.
- **Klein und schnell.** Eine Zähler-App ist auf dem Desktop etwa 8 MB groß und startet in den Benchmarks von NSGo schneller als NSH, MyGo und GPUI.

## Plattformen

| Plattform | Zeichnet mit | Status |
| --- | --- | --- |
| macOS | AppKit und Core Text | Läuft |
| iOS und App Clips | UIKit und Core Text | Läuft im Simulator und auf Geräten; App Clips im Simulator |
| tvOS | UIKit, gesteuert über den Fokus der Fernbedienung | Läuft im Simulator |
| watchOS | SwiftUI und Core Graphics | Läuft im Simulator |
| Android | Android Views | Läuft auf arm64-Geräten und x86_64-Emulatoren |
| Windows | Direct2D und DirectWrite | Läuft |
| Linux | GTK 4, Pango und Cairo | Läuft |
| Web | DOM-Elemente, aus WebAssembly | Läuft |
| HarmonyOS | ArkUI | Vorschau |
| Mini-Programme für WeChat, QQ, Alipay und Douyin | Die Views der jeweiligen Plattform | Experimentell, innerhalb ihrer 2-MB-Grenze |

Apps lassen sich auch in eigene iOS-, Android-, Go-Desktop- und Web-Apps einbetten. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) beschreibt, wie jede Zielplattform ihr System erreicht.

## Erste Schritte

Installieren Sie mit [Go](https://go.dev/dl/) 1.27.3 oder neuer, und Xcode für iOS, tvOS und watchOS, `nsgo` ([andere Wege](docs/public/en/getting-started.md#install-nsgo)), erstellen Sie dann eine App und führen Sie sie mit Hot Reload aus:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # -template tsx für eine TSX-App
cd hello
nsgo dev    # -target web, ios_simulator, tvos_simulator, watchos_simulator, android oder miniprogram_wechat, um sie anderswo auszuführen
nsgo doctor # welche Zielplattformen dieser Rechner baut und ausführt, und was den anderen fehlt
```

Eine App besteht aus Go-Komponenten, die Elemente zurückgeben, wobei Signale aktualisieren, was sie liest:

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

Ihre Tests bedienen die App wie ein Nutzer, und `nsgo test` führt sie headless oder, mit `-target`, auf einer Plattform aus:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Um den Stand von `main` vor einem Release auszuprobieren, führen Sie `go get github.com/nsgo-dev/nsgo@main` aus.

## Status

NSGo ist noch früh in der Entwicklung, und seine API ist noch nicht stabil. Die [Releases](https://github.com/nsgo-dev/nsgo/releases) listen auf, was sich mit jedem Release geändert hat, und die [Roadmap](docs/roadmap.md), was noch fehlt.

## Dokumentation

Die [Dokumentation](https://nsgo.dev/de/docs/) behandelt Komponenten und Zustand, Layout, Styles, Navigation, eine Komponentenreferenz, TSX-Apps, Lokalisierung, Tests, Einbettung, Bauen und Ausliefern; die [Live-Demo](https://nsgo.dev/gallery/) führt die Galerie im Browser aus.

Coding-Agents lernen NSGo über seinen Skill, `bunx skills add nsgo-dev/nsgo`, und lesen die Dokumentation als Markdown über [llms.txt](https://nsgo.dev/llms.txt).

## Entwicklung

```sh
go tool nsgo dev examples/gallery   # die Galerie ausführen
go tool nsgo check                  # jedes Paket prüfen
go tool nsgo test                   # die Tests jedes Pakets
```

[Mitwirken](https://nsgo.dev/de/docs/contributing/) beschreibt die restliche Einrichtung. Lesen Sie [ARCHITECTURE.md](ARCHITECTURE.md) und [AGENTS.md](AGENTS.md), bevor Sie Code ändern, und [CHANGELOG.md](CHANGELOG.md) für die Änderungen seit dem letzten Release.

## Community

Fragen, Ideen und alles, was Sie mit NSGo bauen, sind in nsgo-dev-community willkommen, der Telegram-Gruppe [@nsgo_dev](https://t.me/nsgo_dev). Melden Sie Fehler und schlagen Sie Änderungen in den [Issues](https://github.com/nsgo-dev/nsgo/issues) vor.

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="Der QR-Code der Telegram-Gruppe @nsgo_dev" width="200"></a>
</p>

## Sponsoren

[Depot](https://depot.dev) sponsert die CI von NSGo: Seine Runner bauen und testen NSGo unter Linux, macOS und Windows.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Lizenz

Lizenziert unter der [Apache License, Version 2.0](LICENSE). NSGo ist eine Portierung von NSH, und [NOTICE](NOTICE) hält die Namensnennung fest, die diese Portierung schuldet.

Sofern Sie nicht ausdrücklich etwas anderes angeben, wird jeder Beitrag, den Sie absichtlich zur Aufnahme in NSGo einreichen, wie in der Apache-2.0-Lizenz definiert, unter Apache-2.0 lizenziert, ohne zusätzliche Bedingungen.
