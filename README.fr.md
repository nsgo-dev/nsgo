<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/banner/en-dark.webp">
    <img src=".github/banner/en-light.webp" alt="NSGo : une interface native sur chaque plateforme, écrite en Go" width="100%">
  </picture>
</h1>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">简体中文</a> |
  <a href="README.zh-TW.md">繁體中文</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <strong>Français</strong> |
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
  Un moteur d’interface native multiplateforme avec un backend Go : la logique de l’app, le graphe réactif et la mise en page s’exécutent en Go, et chaque plateforme dessine avec sa propre interface native.<br>
  Il suit la conception de <a href="https://nsh.dev">Native Shell (NSH)</a>, pour les équipes qui veulent le modèle de NSH sans écrire de Rust.
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/desktop-en-dark.jpg">
    <img src="website/src/assets/screens/desktop-en-light.jpg" alt="La démo Stocks de la galerie sur macOS" width="75%">
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/screens/phone-en-dark.jpg">
    <img src="website/src/assets/screens/phone-en-light.jpg" alt="La liste des composants de la galerie sur un téléphone" width="21.6%">
  </picture>
</p>

## Points forts

- **Vues natives.** Le texte, les méthodes de saisie, le défilement, les menus et l’accessibilité sont ceux de chaque plateforme, et ce qu’une plateforme possède, comme Liquid Glass, les autres l’imitent.
- **Toutes les plateformes depuis un seul Mac.** Go compile chaque cible, avec Xcode pour iOS, tvOS et watchOS et un NDK uniquement pour l’émulateur Android x86_64 : ni SDK Android, ni JDK, ni GTK, ni chaîne d’outils Windows.
- **Mises à jour fines.** Les composants s’exécutent une seule fois et les signaux ne mettent à jour que ce qui les lit, avec une mise en page flexbox et grid, des styles proches de CSS, des classes Tailwind, des thèmes et des animations calées sur l’horloge d’affichage de la plateforme.
- **Composants.** `base` propose des composants sans style, des boutons à une `DataTable` d’un million de lignes, en passant par des graphiques, des diagrammes de flux, un éditeur de code, un éditeur de texte enrichi et un terminal, et `ui` leur donne l’apparence de NSGo.
- **Services de la plateforme.** Navigation, notifications, autorisations, sélecteurs, partage, retours haptiques, achats intégrés, widgets et Live Activities, et sur ordinateur, fenêtres, panneaux, menus, icônes de la barre d’état et raccourcis globaux.
- **Localisé.** Catalogues de messages YAML avec pluriels, nombres et dates CLDR, et mise en page de droite à gauche.
- **Apps TSX.** Écrivez des apps en TSX comme avec NSH, avec des mises à jour à chaud qui conservent l’état, et distribuez-les sous forme de paquets signés et isolés qu’une app Go exécute.
- **Des tests qui agissent comme les utilisateurs.** `nsgotest` appuie, tape et lit ce qui est à l’écran, sans interface ou sur chaque plateforme avec saisie native et captures d’écran.
- **L’outil `nsgo`.** Créez des apps, exécutez-les avec rechargement à chaud et DevTools, et empaquetez-les pour les boutiques d’applications, Flathub et le téléchargement direct avec mises à jour.
- **Léger et rapide.** Une app de compteur pèse environ 8 Mo sur ordinateur et démarre plus vite que NSH, MyGo et GPUI dans les benchmarks de NSGo.

## Plateformes

| Plateforme | Dessine avec | État |
| --- | --- | --- |
| macOS | AppKit et Core Text | Fonctionne |
| iOS et App Clips | UIKit et Core Text | Fonctionne dans le simulateur et sur appareil ; App Clips dans le simulateur |
| tvOS | UIKit, piloté par le focus de la télécommande | Fonctionne dans le simulateur |
| watchOS | SwiftUI et Core Graphics | Fonctionne dans le simulateur |
| Android | Android Views | Fonctionne sur appareils arm64 et émulateurs x86_64 |
| Windows | Direct2D et DirectWrite | Fonctionne |
| Linux | GTK 4, Pango et Cairo | Fonctionne |
| Web | Éléments DOM, depuis WebAssembly | Fonctionne |
| HarmonyOS | ArkUI | Aperçu |
| Mini-programmes WeChat, QQ, Alipay et Douyin | Les vues de chaque plateforme | Expérimental, dans leur limite de 2 Mo |

Les apps s’intègrent aussi dans des apps iOS, Android, de bureau Go et web existantes. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) explique comment chaque cible atteint son système.

## Démarrer

Avec [Go](https://go.dev/dl/) 1.27.3 ou plus récent, et Xcode pour iOS, tvOS et watchOS, installez `nsgo` ([autres méthodes](docs/public/en/getting-started.md#install-nsgo)), puis créez une app et exécutez-la avec rechargement à chaud :

```sh
brew install nsgo-dev/tap/nsgo
nsgo init -module example.com/hello hello   # -template tsx pour une app TSX
cd hello
nsgo dev    # -target web, ios_simulator, tvos_simulator, watchos_simulator, android ou miniprogram_wechat pour l’exécuter ailleurs
nsgo doctor # les cibles que cette machine compile et exécute, et ce qui manque aux autres
```

Une app est faite de composants Go qui renvoient des éléments, et les signaux mettent à jour ce qui les lit :

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

Ses tests utilisent l’app comme un utilisateur, et `nsgo test` les exécute sans interface ou, avec `-target`, sur une plateforme :

```go
func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
```

Pour essayer ce qui est sur `main` avant sa publication, lancez `go get github.com/nsgo-dev/nsgo@main`.

## État

NSGo en est à ses débuts, et son API n’est pas encore stable. Les [Releases](https://github.com/nsgo-dev/nsgo/releases) listent ce que chaque version a changé, et la [feuille de route](docs/roadmap.md) ce qui manque encore.

## Documentation

La [documentation](https://nsgo.dev/fr/docs/) couvre les composants et l’état, la mise en page, les styles, la navigation, une référence des composants, les apps TSX, la localisation, les tests, l’intégration, la compilation et la distribution ; la [démo en ligne](https://nsgo.dev/gallery/) exécute la galerie dans le navigateur.

Les agents de code apprennent NSGo grâce à sa skill, `bunx skills add nsgo-dev/nsgo`, et lisent la documentation en Markdown depuis [llms.txt](https://nsgo.dev/llms.txt).

## Développement

```sh
go tool nsgo dev examples/gallery   # lancer la galerie
go tool nsgo check                  # vérifier chaque paquet
go tool nsgo test                   # les tests de chaque paquet
```

[Contribuer](https://nsgo.dev/fr/docs/contributing/) décrit le reste de la configuration. Lisez [ARCHITECTURE.md](ARCHITECTURE.md) et [AGENTS.md](AGENTS.md) avant de modifier le code, et [CHANGELOG.md](CHANGELOG.md) pour ce qui a changé depuis la dernière version.

## Communauté

Questions, idées et ce que vous construisez avec NSGo sont les bienvenus dans nsgo-dev-community, le groupe Telegram [@nsgo_dev](https://t.me/nsgo_dev). Signalez les bugs et proposez des changements dans les [issues](https://github.com/nsgo-dev/nsgo/issues).

<p align="center">
  <a href="https://t.me/nsgo_dev"><img src="docs/assets/telegram-qr.png" alt="Le QR code du groupe Telegram @nsgo_dev" width="200"></a>
</p>

## Sponsors

[Depot](https://depot.dev) sponsorise la CI de NSGo : ses runners compilent et testent NSGo sous Linux, macOS et Windows.

<p align="center">
  <a href="https://depot.dev"><picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/sponsors/depot-dark.svg">
    <img src="website/src/assets/sponsors/depot-light.svg" alt="Depot" height="40">
  </picture></a>
</p>

## Licence

Distribué sous la [licence Apache, version 2.0](LICENSE). NSGo est un portage de NSH, et [NOTICE](NOTICE) consigne l’attribution que ce portage lui doit.

Sauf mention explicite contraire de votre part, toute contribution que vous soumettez intentionnellement pour inclusion dans NSGo, au sens de la licence Apache-2.0, est placée sous licence Apache-2.0, sans conditions supplémentaires.
