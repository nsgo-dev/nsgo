package base

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestButtonWithoutActionKeepsItsPressAndFocus(t *testing.T) {
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return Button{TestID: "button", Style: core.Style{Width: core.Pt(100), Height: core.Pt(30)}}
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS}))
	app.Key("Tab")
	if !app.Find("id:button").Focused {
		t.Fatal("a button without an action is not a keyboard stop")
	}
	app.Key("Space")
	app.Press("id:button")
}
