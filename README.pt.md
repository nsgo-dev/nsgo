<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: UI nativa em toda plataforma, escrita em Go" width="100%">
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
  <strong>Português</strong> |
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
  Um motor de UI nativa multiplataforma com backend em Go: a lógica do app, o grafo reativo e o layout rodam em Go, e cada plataforma desenha com sua própria UI nativa.<br>
  Ele segue o design do <a href="https://nsh.dev">Native Shell (NSH)</a>, para equipes que querem o modelo do NSH sem escrever Rust.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="A demo Stocks da galeria no macOS" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="A lista de componentes da galeria em um celular" width="21.6%">
  </picture>
</p>

## Destaques

- **Views nativas.** Texto, métodos de entrada, rolagem, menus e acessibilidade são os de cada plataforma, e o que uma plataforma tem, como o Liquid Glass, as outras imitam.
- **Todas as plataformas a partir de um Mac.** O Go compila todos os alvos, com Xcode para iOS, tvOS e watchOS e um NDK apenas para o emulador Android x86_64: sem Android SDK, JDK, GTK ou toolchain do Windows.
- **Atualizações granulares.** Os componentes rodam uma vez e os signals atualizam só o que os lê, com layout flexbox e grid, estilos parecidos com CSS, classes Tailwind, temas e animações no relógio de quadros da plataforma.
- **Componentes.** `base` oferece componentes sem estilo, de botões a uma `DataTable` de um milhão de linhas, gráficos, diagramas de fluxo, um editor de código, um editor de texto rico e um terminal, e `ui` dá a eles o visual do NSGo.
- **Serviços da plataforma.** Navegação, notificações, permissões, seletores, compartilhamento, feedback háptico, compras no app, widgets e Live Activities, e nos desktops janelas, painéis, menus, ícones na bandeja e atalhos globais.
- **Localizado.** Catálogos de mensagens em YAML com plurais, números e datas do CLDR, e layout da direita para a esquerda.
- **Apps TSX.** Escreva apps em TSX como no NSH, com atualizações a quente que preservam o estado, e distribua-os como pacotes assinados e em sandbox que um app Go executa.
- **Testes que agem como usuários.** O `nsgotest` pressiona, digita e lê o que está na tela, em modo headless ou em cada plataforma com entrada nativa e capturas de tela.
- **A ferramenta `nsgo`.** Crie apps, rode-os com hot reload e DevTools, e empacote-os para lojas de apps, Flathub e downloads diretos com atualizações.
- **Pequeno e rápido.** Um app de contador fica com cerca de 8 MB nos desktops e inicia mais rápido que NSH, MyGo e GPUI nos benchmarks do NSGo.

## Plataformas

| Plataforma | Desenha com | Status |
| --- | --- | --- |
| macOS | AppKit e Core Text | Funciona |
| iOS e App Clips | UIKit e Core Text | Funciona no simulador e em dispositivos; App Clips no simulador |
| tvOS | UIKit, guiado pelo foco do controle remoto | Funciona no simulador |
| watchOS | SwiftUI e Core Graphics | Funciona no simulador |
| Android | Android Views | Funciona em dispositivos arm64 e emuladores x86_64 |
| Windows | Direct2D e DirectWrite | Funciona |
| Linux | GTK 4, Pango e Cairo | Funciona |
| Web | Elementos DOM, a partir de WebAssembly | Funciona |
| HarmonyOS | ArkUI | Prévia |
| Miniprogramas WeChat, QQ, Alipay e Douyin | As views de cada plataforma | Experimental, dentro do limite de 2 MB |

Os apps também podem ser incorporados em apps iOS, Android, desktop em Go e web próprios. O [ARCHITECTURE.md](ARCHITECTURE.md#platforms) explica como cada alvo chega ao seu sistema.

## Primeiros passos

Com o [Go](https://go.dev/dl/) 1.27.3 ou superior, e o Xcode para iOS, tvOS e watchOS, instale o `nsgo` ([outras formas](docs/public/en/getting-started.md#install-nsgo)), depois crie um app e rode-o com hot reload:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # -template tsx para um app TSX
cd hello
nsgo dev    # -target web, ios_simulator, tvos_simulator, watchos_simulator, android ou miniprogram_wechat para rodar em outro lugar
nsgo doctor # quais alvos esta máquina compila e roda, e o que falta para os outros
```

Um app é feito de componentes Go que retornam elementos, com signals atualizando o que os lê:

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

Seus testes usam o app como um usuário, e `nsgo test` os executa em modo headless ou, com `-target`, em uma plataforma:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Para experimentar o que está na `main` antes de ser lançado, rode `go get github.com/nsgo-dev/nsgo@main`.

## Status

O NSGo está no início, e sua API ainda não é estável. As [Releases](https://github.com/nsgo-dev/nsgo/releases) listam o que cada versão mudou, e o [roadmap](docs/roadmap.md) o que ainda falta.

## Documentação

A [documentação](https://nsgo.dev/pt/docs/) cobre componentes e estado, layout, estilos, navegação, uma referência de componentes, apps TSX, localização, testes, incorporação, build e distribuição; a [demo ao vivo](https://nsgo.dev/gallery/) roda a galeria no navegador.

Agentes de código aprendem NSGo pela sua skill, `bunx skills add nsgo-dev/nsgo`, e leem a documentação em Markdown a partir do [llms.txt](https://nsgo.dev/llms.txt).

## Desenvolvimento

```sh
go tool nsgo dev examples/gallery   # rodar a galeria
go tool nsgo check                  # verificar todos os pacotes
go tool nsgo test                   # os testes de todos os pacotes
```

[Como contribuir](https://nsgo.dev/pt/docs/contributing/) traz o restante da configuração. Leia o [ARCHITECTURE.md](ARCHITECTURE.md) e o [AGENTS.md](AGENTS.md) antes de alterar o código, e o [CHANGELOG.md](CHANGELOG.md) para ver o que mudou desde a última versão.

## Comunidade

Perguntas, ideias e o que você constrói com NSGo são bem-vindos na nsgo-dev-community, o grupo no Telegram [@nsgo_dev](https://t.me/nsgo_dev). Relate bugs e proponha mudanças nas [issues](https://github.com/nsgo-dev/nsgo/issues).

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="O QR code do grupo no Telegram @nsgo_dev" width="200"></a>
</p>

## Patrocinadores

A [Depot](https://depot.dev) patrocina a CI do NSGo: seus runners compilam e testam o NSGo no Linux, macOS e Windows.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Licença

Licenciado sob a [Apache License, Version 2.0](LICENSE). O NSGo é um port do NSH, e o [NOTICE](NOTICE) registra a atribuição que esse port deve.

Salvo declaração explícita em contrário, qualquer contribuição enviada intencionalmente por você para inclusão no NSGo, conforme definido na licença Apache-2.0, será licenciada sob a Apache-2.0, sem quaisquer termos ou condições adicionais.
