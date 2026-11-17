// Command inputisolation checks that native tests take only the host's input.
package main

import (
	"strconv"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

func main() { nsgo.Run(Root) }

func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	add := func() { count.Update(func(n int) int { return n + 1 }) }
	cx.OnKey("x", add)
	// The menu bar's key equivalent, which a key's responder never sees.
	cx.SetApplicationMenu([]core.MenuItem{{Label: "Input Isolation", Items: []core.MenuItem{{Label: "Add", Shortcut: "mod+k", OnSelect: add}}}})
	return core.View{TestID: "add", Role: core.RoleButton,
		Style:    core.Style{Width: core.Pt(160), Height: core.Pt(80)},
		OnPress:  func(core.PressEvent) { add() },
		Children: []core.Node{core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }}}}
}
