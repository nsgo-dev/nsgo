package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestPopupCleanupDoesNotTakeTheNextAppsInput leaves a native menu open
// at a subtest's end, then checks the next app's very first input.
func TestPopupCleanupDoesNotTakeTheNextAppsInput(t *testing.T) {
	if target := nsgotest.Target(); target != "" && target != "macos" && target != "windows" && target != "linux" {
		t.Skip("needs native desktop menus and several apps")
	}
	for _, input := range []string{"click", "key"} {
		t.Run(input, func(t *testing.T) {
			t.Run("menu", func(t *testing.T) {
				app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
					return core.View{TestID: "menu-trigger", Style: core.Style{Width: core.Pt(160), Height: core.Pt(80)},
						OnContextMenu: func(e core.ContextMenuEvent) {
							desktop.PopupMenu(cx, []desktop.MenuItem{{Label: "Copy"}}, e.PagePosition, nil)
						}}
				}, nsgotest.Environment(desktopEnv))
				app.RightClick("id:menu-trigger")
				if len(app.PopupMenu) != 1 {
					t.Fatal("the context menu did not open")
				}
			})
			t.Run("next app", func(t *testing.T) {
				app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
					received := core.NewSignal(cx, false)
					cx.OnKey("mod+shift+c", func() { received.Set(true) })
					return core.View{TestID: "button", Style: core.Style{Width: core.Pt(200), Height: core.Pt(80)},
						OnPress: func(core.PressEvent) { received.Set(true) },
						Children: []core.Node{core.Text{TestID: "input-result", ContentFunc: func() string {
							if received.Get() {
								return "received"
							}
							return "waiting"
						}}}}
				}, nsgotest.Environment(desktopEnv))
				if input == "click" {
					app.Press("id:button")
				} else {
					app.Key("mod+shift+c")
				}
				if got := app.Find("id:input-result").Text; got != "received" {
					t.Fatalf("the first %s left %q; the previous app's menu took it", input, got)
				}
			})
		})
	}
}
