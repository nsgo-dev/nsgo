// Command settle is the native regression fixture for waiting on a slow dialog commit.
package main

import (
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func main() { nsgo.Run(Root) }

func Root(cx *core.Scope) core.Node {
	open := core.NewSignal(cx, false)
	return core.View{Children: []core.Node{
		core.View{TestID: "open", Role: core.RoleButton,
			Style:    core.Style{Width: core.Pt(120), Height: core.Pt(44)},
			OnPress:  func(core.PressEvent) { cx.Post(func() { open.Set(true) }) },
			Children: []core.Node{core.Text{Content: "Open"}}},
		core.Show{When: open.Get, Then: func(cx *core.Scope) core.Node {
			// Model the first construction of a large subtree taking longer
			// than Settle's 100 ms quiet interval, without app-side barriers.
			time.Sleep(400 * time.Millisecond)
			return ui.Dialog{Open: func() bool { return true }, Bare: true, TestID: "guide",
				Style: core.Style{Width: core.Pct(100), Height: core.Pct(100),
					MaxWidth: core.Pct(100), MaxHeight: core.Pct(100)},
				Children: []core.Node{core.Text{Content: "1 / 3"}}}
		}},
	}}
}
