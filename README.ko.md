<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: Go로 작성하는 모든 플랫폼의 네이티브 UI" width="100%">
  </picture>
</h1>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">简体中文</a> |
  <a href="README.zh-TW.md">繁體中文</a> |
  <a href="README.ja.md">日本語</a> |
  <strong>한국어</strong> |
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
  Go 백엔드를 갖춘 크로스 플랫폼 네이티브 UI 엔진입니다. 앱 로직, 반응형 그래프, 레이아웃은 Go에서 실행되고, 화면은 각 플랫폼의 네이티브 UI로 그립니다.<br>
  <a href="https://nsh.dev">Native Shell(NSH)</a>의 설계를 따르며, Rust를 쓰지 않고 NSH의 모델을 원하는 팀을 위한 것입니다.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="macOS에서 실행한 갤러리의 Stocks 데모" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="휴대폰에서 본 갤러리의 컴포넌트 목록" width="21.6%">
  </picture>
</p>

## 주요 특징

- **네이티브 뷰.** 텍스트, 입력기, 스크롤, 메뉴, 접근성은 모두 각 플랫폼 고유의 것을 쓰며, Liquid Glass처럼 한 플랫폼에만 있는 것은 다른 플랫폼이 따라 구현합니다.
- **Mac 한 대로 모든 플랫폼을.** Go가 모든 대상을 빌드하며, iOS, tvOS, watchOS에는 Xcode가, x86_64 Android 에뮬레이터에만 NDK가 필요합니다. Android SDK, JDK, GTK, Windows 툴체인은 필요 없습니다.
- **세밀한 업데이트.** 컴포넌트는 한 번만 실행되고 시그널은 자신을 읽는 부분만 업데이트합니다. flexbox와 grid 레이아웃, CSS와 비슷한 스타일, Tailwind 클래스, 테마, 플랫폼의 프레임 클록에 맞춘 애니메이션을 지원합니다.
- **컴포넌트.** `base`는 버튼부터 백만 행짜리 `DataTable`, 차트, 플로 다이어그램, 코드 편집기, 서식 있는 텍스트 편집기, 터미널까지 스타일 없는 컴포넌트를 제공하고, `ui`는 여기에 NSGo의 디자인을 입힙니다.
- **플랫폼 서비스.** 내비게이션, 알림, 권한, 선택기, 공유, 햅틱, 인앱 결제, 위젯, 실시간 현황(Live Activities)을 제공하며, 데스크톱에서는 창, 패널, 메뉴, 트레이 아이콘, 전역 단축키도 지원합니다.
- **현지화.** CLDR 복수형, 숫자, 날짜를 지원하는 YAML 메시지 카탈로그와 오른쪽에서 왼쪽으로 흐르는 레이아웃.
- **TSX 앱.** NSH처럼 TSX로 앱을 작성하고, 상태를 유지하는 핫 업데이트를 쓰며, Go 앱이 실행하는 서명된 샌드박스 패키지로 배포할 수 있습니다.
- **사용자처럼 동작하는 테스트.** `nsgotest`는 화면의 요소를 누르고, 입력하고, 읽습니다. 헤드리스로 실행하거나 각 플랫폼에서 네이티브 입력과 스크린샷으로 실행할 수 있습니다.
- **`nsgo` 도구.** 앱을 생성하고, 핫 리로드와 DevTools로 실행하고, 앱 스토어, Flathub, 업데이트를 지원하는 직접 다운로드용으로 패키징합니다.
- **작고 빠름.** 카운터 앱은 데스크톱에서 약 8 MB로 패키징되며, NSGo의 벤치마크에서 NSH, MyGo, GPUI보다 빠르게 시작합니다.

## 플랫폼

| 플랫폼 | 그리기 | 상태 |
| --- | --- | --- |
| macOS | AppKit과 Core Text | 실행됨 |
| iOS 및 App Clips | UIKit과 Core Text | 시뮬레이터와 기기에서 실행됨, App Clips는 시뮬레이터에서 실행됨 |
| tvOS | UIKit, 리모컨 포커스로 조작 | 시뮬레이터에서 실행됨 |
| watchOS | SwiftUI와 Core Graphics | 시뮬레이터에서 실행됨 |
| Android | Android View | arm64 기기와 x86_64 에뮬레이터에서 실행됨 |
| Windows | Direct2D와 DirectWrite | 실행됨 |
| Linux | GTK 4, Pango, Cairo | 실행됨 |
| 웹 | WebAssembly로 구동하는 DOM 요소 | 실행됨 |
| HarmonyOS | ArkUI | 미리보기 |
| WeChat, QQ, Alipay, Douyin 미니 프로그램 | 각 플랫폼의 뷰 | 실험적, 2 MB 제한 안에서 동작 |

앱은 기존 iOS, Android, Go 데스크톱, 웹 앱에 임베드할 수도 있습니다. 각 대상이 시스템에 어떻게 연결되는지는 [ARCHITECTURE.md](ARCHITECTURE.md#platforms)를 참고하세요.

## 시작하기

[Go](https://go.dev/dl/) 1.27.3 이상(iOS, tvOS, watchOS에는 Xcode도)을 준비하고 `nsgo`를 설치한 다음([다른 방법](docs/public/en/getting-started.md#install-nsgo)), 앱을 생성해 핫 리로드로 실행합니다:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # TSX 앱은 -template tsx
cd hello
nsgo dev    # 다른 곳에서 실행하려면 -target web, ios_simulator, tvos_simulator, watchos_simulator, android 또는 miniprogram_wechat
nsgo doctor # 이 기기에서 빌드하고 실행할 수 있는 대상과, 나머지 대상에 부족한 것
```

앱은 요소를 반환하는 Go 컴포넌트로 이루어지며, 시그널이 자신을 읽는 부분을 업데이트합니다:

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

테스트는 사용자처럼 앱을 조작하며, `nsgo test`는 이를 헤드리스로 실행하거나 `-target`을 주면 해당 플랫폼에서 실행합니다:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

릴리스 전의 `main`을 써 보려면 `go get github.com/nsgo-dev/nsgo@main`을 실행하세요.

## 현황

NSGo는 초기 단계이며 API는 아직 안정적이지 않습니다. 각 릴리스의 변경 사항은 [Releases](https://github.com/nsgo-dev/nsgo/releases)에, 아직 부족한 부분은 [로드맵](docs/roadmap.md)에 정리되어 있습니다.

## 문서

[문서](https://nsgo.dev/ko/docs/)는 컴포넌트와 상태, 레이아웃, 스타일, 내비게이션, 컴포넌트 레퍼런스, TSX 앱, 현지화, 테스트, 임베딩, 빌드와 배포를 다룹니다. [라이브 데모](https://nsgo.dev/gallery/)에서는 브라우저에서 갤러리를 실행해 볼 수 있습니다.

코딩 에이전트는 NSGo의 스킬(`bunx skills add nsgo-dev/nsgo`)로 NSGo를 익히고, [llms.txt](https://nsgo.dev/llms.txt)에서 문서를 Markdown으로 읽을 수 있습니다.

## 개발

```sh
go tool nsgo dev examples/gallery   # 갤러리 실행
go tool nsgo check                  # 모든 패키지 검사
go tool nsgo test                   # 모든 패키지의 테스트 실행
```

나머지 설정은 [개발 환경 설정](https://nsgo.dev/ko/docs/contributing/)에 있습니다. 코드를 변경하기 전에 [ARCHITECTURE.md](ARCHITECTURE.md)와 [AGENTS.md](AGENTS.md)를 읽고, 지난 릴리스 이후의 변경 사항은 [CHANGELOG.md](CHANGELOG.md)에서 확인하세요.

## 커뮤니티

질문, 아이디어, NSGo로 만든 결과물은 Telegram 그룹 nsgo-dev-community([@nsgo_dev](https://t.me/nsgo_dev))에서 언제든 환영합니다. 버그 신고와 변경 제안은 [issues](https://github.com/nsgo-dev/nsgo/issues)에 남겨 주세요.

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="Telegram 그룹 @nsgo_dev의 QR 코드" width="200"></a>
</p>

## 후원

[Depot](https://depot.dev)이 NSGo의 CI를 후원합니다. Depot의 러너가 Linux, macOS, Windows에서 NSGo를 빌드하고 테스트합니다.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## 라이선스

[Apache License 2.0](LICENSE)에 따라 라이선스가 부여됩니다. NSGo는 NSH의 포트이며, 이에 따른 저작자 표시는 [NOTICE](NOTICE)에 기록되어 있습니다.

명시적으로 달리 밝히지 않는 한, Apache-2.0 라이선스에 정의된 대로 귀하가 NSGo에 포함되도록 의도적으로 제출한 모든 기여는 추가 조건 없이 Apache-2.0에 따라 라이선스가 부여됩니다.
