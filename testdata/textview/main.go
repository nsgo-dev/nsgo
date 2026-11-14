// Command textview shows a styled text view without RenderMath: linked_test.go checks that it links no formulas.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func main() {
	nsgo.Run(func(cx *core.Scope) core.Node { return ui.TextView{Markdown: "Energy $E=mc^2$"} })
}
