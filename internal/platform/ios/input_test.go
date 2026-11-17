//go:build ios && nsgotest

package ios_test

import (
	"testing"
	"time"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/ios"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestInputLayoutAfterPoppingPage(t *testing.T) {
	const wait = 5 * time.Second
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		nav := base.NewNav(cx, "Home")
		return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
			if route == "Home" {
				return base.NavPage{Title: route, Children: []core.Node{
					base.Button{Label: "Open form", TestID: "push", Style: core.Style{Width: core.Pt(160), Height: core.Pt(44)},
						OnPress: func(core.PressEvent) { nav.Push("Form") }},
				}}
			}
			return base.NavPage{Title: route, TestID: "form", Children: []core.Node{
				core.Input{TestID: "field", Label: "Address", Style: core.Style{Height: core.Pt(44)}},
				core.Input{TestID: "notes", Label: "Notes", Multiline: true, Style: core.Style{Height: core.Pt(100)}},
			}}
		})
	})
	for range 3 {
		app.Press("id:push")
		app.WaitFor("id:field", wait)
		app.Type("id:field", "Address")
		app.Type("id:notes", "Notes")
		var callbacks []func() bool
		for _, id := range []string{"field", "notes"} {
			layout, release := ios.RetainInputForTest(id)
			if layout == nil {
				t.Fatalf("the native input %q did not mount", id)
			}
			defer release()
			callbacks = append(callbacks, layout)
		}
		app.Back()
		app.WaitForGone("id:form", wait)
		for _, layout := range callbacks {
			app.WaitUntil(wait, layout)
		}
		app.WaitFor("id:push", wait)
	}
}
