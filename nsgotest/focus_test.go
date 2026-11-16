package nsgotest

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Ref.Focus goes through mount commands, unlike the Focus test step,
// which uses Tab. This catches a model that promises focus the native
// command handler cannot give.
func TestRefFocusMatchesThePlatformsKeyboardStops(t *testing.T) {
	for _, env := range []driver.Environment{
		{OS: driver.OSMacOS}, {OS: driver.OSWindows}, {OS: driver.OSLinux},
		{OS: driver.OSWeb}, {OS: driver.OSIOS}, {OS: driver.OSAndroid},
		{OS: driver.OSIOS, Idiom: driver.IdiomTV},
	} {
		name := string(env.OS)
		if env.Idiom == driver.IdiomTV {
			name += "/tv"
		}
		t.Run(name, func(t *testing.T) {
			var field, button, plain, explicit, blocked, hidden, disabled core.Ref
			pressed := 0
			app := func(cx *core.Scope) core.Node {
				row := core.Style{Width: core.Pt(120), Height: core.Pt(25)}
				return core.View{Children: []core.Node{
					core.Input{TestID: "field", Ref: &field, Style: row},
					core.View{TestID: "button", Ref: &button, Style: row, OnPress: func(core.PressEvent) { pressed++ },
						Class: "focus-visible:bg-[#123456]"},
					core.View{TestID: "plain", Ref: &plain, Style: row},
					core.View{TestID: "explicit", Ref: &explicit, Style: row, Focusable: func() bool { return true }},
					core.View{TestID: "blocked", Ref: &blocked, Style: row, NoFocus: true, OnPress: func(core.PressEvent) { pressed++ }},
					core.View{Style: core.Style{Visibility: core.VisibilityHidden}, Children: []core.Node{
						core.View{Ref: &hidden, Style: row, Focusable: func() bool { return true }},
					}},
					core.View{Ref: &disabled, Style: row, Disabled: func() bool { return true },
						OnPress: func(core.PressEvent) { pressed++ }},
				}}
			}
			a := Start(t, app, Environment(env))
			field.Focus()
			a.Settle()
			if !a.Find("id:field").Focused {
				t.Fatal("Ref.Focus did not focus the input")
			}
			button.Focus()
			a.Settle()
			if !env.FocusesViews() {
				if a.Find("id:button").Focused || !a.Find("id:field").Focused {
					t.Fatal("a touch platform moved focus from the input to an ordinary view")
				}
				return
			}
			if !a.Find("id:button").Focused || a.Find("id:field").Focused {
				t.Fatal("Ref.Focus did not move focus from the input to the button")
			}
			a.Key("Space")
			a.Key("Return")
			if pressed != 2 || a.Find("id:button").Style.Background != driver.RGB(0x123456) {
				t.Fatalf("focused button: presses %d, focus style %#x", pressed, a.Find("id:button").Style.Background)
			}
			plain.Focus()
			blocked.Focus()
			hidden.Focus()
			disabled.Focus()
			field.Blur()
			a.Settle()
			if !a.Find("id:button").Focused {
				t.Fatal("an invalid focus or blur of another view stole the button's focus")
			}
			explicit.Focus()
			a.Settle()
			if !a.Find("id:explicit").Focused || a.Find("id:button").Focused {
				t.Fatal("a Focusable view without a press handler did not take focus")
			}
			explicit.Blur()
			a.Settle()
			if a.Find("id:explicit").Focused {
				t.Fatal("Ref.Blur left its view focused")
			}
		})
	}
}
