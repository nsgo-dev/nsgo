// Command activation is the native keyboard fixture for nsgo test -activate.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

func main() { nsgo.Run(Root) }

// Root gives the keyboard a text field and a Tab stop that counts presses.
func Root(cx *core.Scope) core.Node {
	text := core.NewSignal(cx, "")
	pressed := core.NewSignal(cx, false)
	return core.View{Children: []core.Node{
		core.Input{TestID: "field", Value: text.Get, OnChange: text.Set, Style: core.Style{Width: core.Pt(240), Height: core.Pt(32)}},
		core.View{TestID: "button", Role: core.RoleButton, Style: core.Style{Width: core.Pt(120), Height: core.Pt(32)}, OnPress: func(core.PressEvent) { pressed.Set(true) }, Children: []core.Node{
			core.Text{ContentFunc: func() string {
				if pressed.Get() {
					return "Pressed"
				}
				return "Press"
			}},
		}},
	}}
}
