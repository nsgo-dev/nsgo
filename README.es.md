<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo: UI nativa en cada plataforma, escrita en Go" width="100%">
  </picture>
</h1>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">简体中文</a> |
  <a href="README.zh-TW.md">繁體中文</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <strong>Español</strong> |
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
  Un motor de UI nativa multiplataforma con backend en Go: la lógica de la aplicación, el grafo reactivo y el diseño se ejecutan en Go, y cada plataforma dibuja con su propia UI nativa.<br>
  Sigue el diseño de <a href="https://nsh.dev">Native Shell (NSH)</a>, para equipos que quieren el modelo de NSH sin escribir Rust.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="La demo Stocks de la galería en macOS" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="La lista de componentes de la galería en un teléfono" width="21.6%">
  </picture>
</p>

## Puntos destacados

- **Vistas nativas.** El texto, los métodos de entrada, el desplazamiento, los menús y la accesibilidad son los de cada plataforma, y lo que tiene una sola, como Liquid Glass, las demás lo imitan.
- **Todas las plataformas desde una Mac.** Go compila todos los destinos, con Xcode para iOS, tvOS y watchOS y un NDK solo para el emulador de Android x86_64: sin Android SDK, JDK, GTK ni toolchain de Windows.
- **Actualizaciones de grano fino.** Los componentes se ejecutan una vez y las señales actualizan solo lo que las lee, con diseño flexbox y grid, estilos al estilo de CSS, clases de Tailwind, temas y animaciones al ritmo del reloj de fotogramas de la plataforma.
- **Componentes.** `base` ofrece componentes sin estilo, desde botones hasta una `DataTable` de un millón de filas, gráficos, diagramas de flujo, un editor de código, un editor de texto enriquecido y una terminal, y `ui` les da el aspecto de NSGo.
- **Servicios de la plataforma.** Navegación, notificaciones, permisos, selectores, compartir, hápticos, compras dentro de la app, widgets y Live Activities, y en escritorio ventanas, paneles, menús, iconos en la bandeja y atajos globales.
- **Localización.** Catálogos de mensajes en YAML con plurales, números y fechas de CLDR, y diseño de derecha a izquierda.
- **Apps en TSX.** Escribe apps en TSX como en NSH, con actualizaciones en caliente que conservan el estado, y distribúyelas como paquetes firmados y aislados que ejecuta una app en Go.
- **Pruebas que actúan como los usuarios.** `nsgotest` pulsa, escribe y lee lo que hay en pantalla, sin interfaz o en cada plataforma con entrada nativa y capturas de pantalla.
- **La herramienta `nsgo`.** Crea apps, ejecútalas con recarga en caliente y DevTools, y empaquétalas para tiendas de aplicaciones, Flathub y descargas directas con actualizaciones.
- **Pequeño y rápido.** Una app de contador ocupa unos 8 MB empaquetada en escritorio y arranca más rápido que NSH, MyGo y GPUI en los benchmarks de NSGo.

## Plataformas

| Plataforma | Dibuja con | Estado |
| --- | --- | --- |
| macOS | AppKit y Core Text | Funciona |
| iOS y App Clips | UIKit y Core Text | Funciona en el simulador y en dispositivos; App Clips en el simulador |
| tvOS | UIKit, guiado por el foco del control remoto | Funciona en el simulador |
| watchOS | SwiftUI y Core Graphics | Funciona en el simulador |
| Android | Vistas de Android | Funciona en dispositivos arm64 y emuladores x86_64 |
| Windows | Direct2D y DirectWrite | Funciona |
| Linux | GTK 4, Pango y Cairo | Funciona |
| Web | Elementos del DOM, desde WebAssembly | Funciona |
| HarmonyOS | ArkUI | Vista previa |
| Miniprogramas de WeChat, QQ, Alipay y Douyin | Las vistas de cada plataforma | Experimental, dentro de su límite de 2 MB |

Las apps también se integran en apps propias de iOS, Android, escritorio en Go y web. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) explica cómo llega cada destino a su sistema.

## Primeros pasos

Con [Go](https://go.dev/dl/) 1.27.3 o posterior, y Xcode para iOS, tvOS y watchOS, instala `nsgo` ([otras formas](docs/public/en/getting-started.md#install-nsgo)), luego crea una app y ejecútala con recarga en caliente:

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # -template tsx para una app en TSX
cd hello
nsgo dev    # -target web, ios_simulator, tvos_simulator, watchos_simulator, android o miniprogram_wechat para ejecutarla en otro lugar
nsgo doctor # qué destinos compila y ejecuta esta máquina, y qué les falta a los demás
```

Una app se compone de componentes de Go que devuelven elementos, y las señales actualizan lo que las lee:

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

Sus pruebas usan la app como lo hace un usuario, y `nsgo test` las ejecuta sin interfaz o, con `-target`, en una plataforma:

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Para probar lo que hay en `main` antes de que se publique, ejecuta `go get github.com/nsgo-dev/nsgo@main`.

## Estado

NSGo está en sus inicios y su API aún no es estable. [Releases](https://github.com/nsgo-dev/nsgo/releases) recoge lo que cambió en cada versión, y la [hoja de ruta](docs/roadmap.md), lo que todavía falta.

## Documentación

La [documentación](https://nsgo.dev/es/docs/) cubre componentes y estado, diseño, estilos, navegación, una referencia de componentes, apps en TSX, localización, pruebas, integración, compilación y distribución; la [demo en vivo](https://nsgo.dev/gallery/) ejecuta la galería en el navegador.

Los agentes de programación aprenden NSGo con su skill, `bunx skills add nsgo-dev/nsgo`, y leen la documentación en Markdown desde [llms.txt](https://nsgo.dev/llms.txt).

## Desarrollo

```sh
go tool nsgo dev examples/gallery   # ejecuta la galería
go tool nsgo check                  # revisa todos los paquetes
go tool nsgo test                   # las pruebas de todos los paquetes
```

[Configuración de desarrollo](https://nsgo.dev/es/docs/contributing/) tiene el resto de la configuración. Lee [ARCHITECTURE.md](ARCHITECTURE.md) y [AGENTS.md](AGENTS.md) antes de cambiar código, y [CHANGELOG.md](CHANGELOG.md) para ver qué cambió desde la última versión.

## Comunidad

Las preguntas, ideas y lo que construyas con NSGo son bienvenidos en nsgo-dev-community, el grupo de Telegram [@nsgo_dev](https://t.me/nsgo_dev). Informa de errores y propón cambios en [issues](https://github.com/nsgo-dev/nsgo/issues).

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="El código QR del grupo de Telegram @nsgo_dev" width="200"></a>
</p>

## Patrocinadores

[Depot](https://depot.dev) patrocina la CI de NSGo: sus runners compilan y prueban NSGo en Linux, macOS y Windows.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Licencia

Publicado bajo la [Licencia Apache, versión 2.0](LICENSE). NSGo es un port de NSH, y [NOTICE](NOTICE) recoge la atribución que ese port le debe.

Salvo que indiques explícitamente lo contrario, cualquier contribución que envíes intencionadamente para su inclusión en NSGo, tal como se define en la licencia Apache-2.0, se licenciará bajo Apache-2.0, sin términos ni condiciones adicionales.
