<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: нативный интерфейс на каждой платформе, написанный на Go" width="100%">
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
  <strong>Русский</strong> |
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
  Кроссплатформенный движок нативного UI с бэкендом на Go: логика приложения, реактивный граф и раскладка работают на Go, а каждая платформа рисует своим нативным UI.<br>
  Он следует архитектуре <a href="https://nsh.dev">Native Shell (NSH)</a> — для команд, которым нужна модель NSH без написания кода на Rust.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="Демо Stocks из галереи на macOS" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="Список компонентов галереи на телефоне" width="21.6%">
  </picture>
</p>

## Основное

- **Нативные представления.** Текст, методы ввода, прокрутка, меню и специальные возможности — собственные у каждой платформы, а то, что есть на одной платформе, например Liquid Glass, остальные имитируют.
- **Все платформы с одного Mac.** Go собирает каждую целевую платформу, Xcode нужен для iOS, tvOS и watchOS, а NDK — только для эмулятора Android на x86_64: без Android SDK, JDK, GTK и инструментов сборки Windows.
- **Точечные обновления.** Компоненты выполняются один раз, а сигналы обновляют только то, что их читает; есть раскладка flexbox и grid, стили в духе CSS, классы Tailwind, темы и анимации по частоте кадров платформы.
- **Компоненты.** `base` предлагает нестилизованные компоненты — от кнопок до `DataTable` на миллион строк, графики, блок-схемы, редактор кода, редактор форматированного текста и терминал, а `ui` придаёт им облик NSGo.
- **Сервисы платформы.** Навигация, уведомления, разрешения, средства выбора, «Поделиться», тактильная отдача, встроенные покупки, виджеты и Live Activities, а на десктопах — окна, панели, меню, значки в трее и глобальные сочетания клавиш.
- **Локализация.** Каталоги сообщений в YAML с правилами множественного числа, числами и датами CLDR, а также раскладка справа налево.
- **Приложения на TSX.** Пишите приложения на TSX, как в NSH, с горячими обновлениями, сохраняющими состояние, и поставляйте их как подписанные изолированные пакеты, которые запускает Go-приложение.
- **Тесты, которые действуют как пользователь.** `nsgotest` нажимает, печатает и читает то, что на экране, — в headless-режиме или на каждой платформе с нативным вводом и скриншотами.
- **Инструмент `nsgo`.** Создавайте приложения, запускайте их с горячей перезагрузкой и DevTools и упаковывайте для магазинов приложений, Flathub и прямой загрузки с обновлениями.
- **Компактный и быстрый.** Приложение-счётчик весит около 8 МБ на десктопах и в бенчмарках NSGo запускается быстрее, чем NSH, MyGo и GPUI.

## Платформы

| Платформа | Чем рисует | Статус |
| --- | --- | --- |
| macOS | AppKit и Core Text | Работает |
| iOS и App Clips | UIKit и Core Text | Работает в симуляторе и на устройствах; App Clips — в симуляторе |
| tvOS | UIKit, управление фокусом с пульта | Работает в симуляторе |
| watchOS | SwiftUI и Core Graphics | Работает в симуляторе |
| Android | Android Views | Работает на устройствах arm64 и эмуляторах x86_64 |
| Windows | Direct2D и DirectWrite | Работает |
| Linux | GTK 4, Pango и Cairo | Работает |
| Web | Элементы DOM из WebAssembly | Работает |
| HarmonyOS | ArkUI | Предпросмотр |
| Мини-программы WeChat, QQ, Alipay и Douyin | Представления каждой платформы | Экспериментально, в пределах их лимита в 2 МБ |

Приложения также встраиваются в собственные iOS-, Android-, десктопные Go- и веб-приложения. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) описывает, как каждая целевая платформа обращается к своей системе.

## Начало работы

С [Go](https://go.dev/dl/) 1.27.3 или новее, а также Xcode для iOS, tvOS и watchOS, установите `nsgo` ([другие способы](docs/public/en/getting-started.md#install-nsgo)), затем создайте приложение и запустите его с горячей перезагрузкой:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # -template tsx для приложения на TSX
cd hello
nsgo dev    # -target web, ios_simulator, tvos_simulator, watchos_simulator, android или miniprogram_wechat для запуска в другом месте
nsgo doctor # какие целевые платформы эта машина собирает и запускает и чего не хватает остальным
```

Приложение — это Go-компоненты, возвращающие элементы, а сигналы обновляют то, что их читает:

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

Его тесты работают с приложением так же, как пользователь, а `nsgo test` запускает их в headless-режиме или, с `-target`, на платформе:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Чтобы попробовать то, что есть в `main`, до выхода релиза, выполните `go get github.com/nsgo-dev/nsgo@main`.

## Статус

NSGo находится на ранней стадии, и его API пока не стабилен. В [Releases](https://github.com/nsgo-dev/nsgo/releases) перечислено, что изменилось в каждом релизе, а в [дорожной карте](docs/roadmap.md) — чего ещё не хватает.

## Документация

[Документация](https://nsgo.dev/ru/docs/) охватывает компоненты и состояние, раскладку, стили, навигацию, справочник компонентов, приложения на TSX, локализацию, тестирование, встраивание, сборку и поставку; [живое демо](https://nsgo.dev/gallery/) запускает галерею в браузере.

ИИ-агенты для программирования изучают NSGo по его навыку, `bunx skills add nsgo-dev/nsgo`, и читают документацию в Markdown через [llms.txt](https://nsgo.dev/llms.txt).

## Разработка

```sh
go tool nsgo dev examples/gallery   # запустить галерею
go tool nsgo check                  # проверить каждый пакет
go tool nsgo test                   # тесты каждого пакета
```

В разделе [«Участие в разработке»](https://nsgo.dev/ru/docs/contributing/) описана остальная настройка. Прочитайте [ARCHITECTURE.md](ARCHITECTURE.md) и [AGENTS.md](AGENTS.md), прежде чем менять код, а в [CHANGELOG.md](CHANGELOG.md) смотрите, что изменилось с последнего релиза.

## Сообщество

Вопросы, идеи и то, что вы создаёте с NSGo, приветствуются в nsgo-dev-community — группе в Telegram [@nsgo_dev](https://t.me/nsgo_dev). Сообщайте об ошибках и предлагайте изменения в [issues](https://github.com/nsgo-dev/nsgo/issues).

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="QR-код группы в Telegram @nsgo_dev" width="200"></a>
</p>

## Спонсоры

[Depot](https://depot.dev) спонсирует CI NSGo: его раннеры собирают и тестируют NSGo на Linux, macOS и Windows.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Лицензия

Распространяется по [лицензии Apache версии 2.0](LICENSE). NSGo — порт NSH, и в [NOTICE](NOTICE) указано авторство, которым этот порт обязан.

Если вы явно не укажете иное, любой вклад, намеренно отправленный вами для включения в NSGo, как определено в лицензии Apache-2.0, лицензируется по Apache-2.0 без каких-либо дополнительных условий.
