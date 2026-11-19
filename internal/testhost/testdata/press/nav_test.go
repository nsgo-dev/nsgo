package press_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

// A press on a page that a press just pushed, where the pushing button was, is a single click that reaches the new page's button, and so is one on the page below right after a pop (#854). The window system counts a second press there within its double-click time as a double click and may send it to the view that took the first.
func TestPressAfterNavigationPushAndPop(t *testing.T) {
	opened, pressed := 0, 0
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		nav := base.NewNav(cx, "home")
		return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
			if route == "home" {
				return ui.NavPage(cx, base.NavPage{Title: "Home", Children: []core.Node{
					ui.Button{Text: "Open", TestID: "open", OnPress: func() { opened++; nav.Push("next") }},
				}}, ui.NavHeader{})
			}
			return ui.NavPage(cx, base.NavPage{Title: "Next", Children: []core.Node{
				ui.Button{Text: "Target", TestID: "target", OnPress: func() { pressed++ }},
			}}, ui.NavHeader{OnBack: func() { nav.Pop(1) }, BackLabel: "Back"})
		})
	})
	for i := 1; i <= 20; i++ {
		app.Press("id:open")
		if opened != i {
			t.Fatalf("round %d: the press after a pop opened %d pages, want %d", i, opened, i)
		}
		app.WaitFor("id:target", time.Second)
		app.Press("id:target")
		if pressed != i {
			t.Fatalf("round %d: the press after a push reached the page %d times, want %d", i, pressed, i)
		}
		app.Back()
		app.WaitFor("id:open", time.Second)
	}
}
