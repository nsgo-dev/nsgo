package press_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

// This fixture runs through actual pointer and keyboard input on each desktop.
func TestCheckboxPressModifiersReachTheApp(t *testing.T) {
	switch nsgotest.Target() {
	case "", "macos", "windows", "linux", "web":
	default:
		t.Skip("physical-keyboard activation on mobile is tracked in the roadmap")
	}
	var got []core.PressEvent
	legacy := 0
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var children []core.Node
		for _, styled := range []bool{false, true} {
			checked := core.NewSignal(cx, false)
			take := func(next bool, e core.PressEvent) { checked.Set(next); got = append(got, e) }
			style := core.Style{Width: core.Pt(220), Height: core.Pt(40)}
			if styled {
				children = append(children, ui.Checkbox{TestID: "styled", Text: "Styled", Checked: checked.Get,
					Style: style, OnChange: func(bool) { legacy++ }, OnChangeEvent: take})
			} else {
				children = append(children, base.Checkbox{TestID: "base", Label: "Base", Checked: checked.Get,
					Style: style, OnChange: func(bool) { legacy++ }, OnChangeEvent: take,
					Children: []core.Node{core.Text{Content: "Base"}}})
			}
		}
		return core.View{Style: core.Style{Padding: core.All(20)}, Children: children}
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}), nsgotest.Size(300, 200))
	check := func(mods core.Modifiers, keyboard bool) {
		t.Helper()
		if len(got) != 1 || got[0].Modifiers != mods || got[0].Keyboard != keyboard || (got[0].Clicks < 1 || keyboard && got[0].Clicks != 1) || legacy != 0 {
			t.Fatalf("press=%+v legacy=%d, want one with %v keyboard=%v", got, legacy, mods, keyboard)
		}
		got = nil
	}
	for _, id := range []string{"base", "styled"} {
		app.PressWith("id:"+id, core.Modifiers{Shift: true, Alt: true})
		check(core.Modifiers{Shift: true, Alt: true}, false)
		app.Press("id:" + id)
		check(core.Modifiers{}, false)
		app.Focus("id:" + id)
		app.Key("shift+space")
		check(core.Modifiers{Shift: true}, true)
		app.Key("enter")
		check(core.Modifiers{}, true)
		if app.Find("id:" + id).Props.Checked {
			t.Fatal("four activations did not restore unchecked")
		}
	}
}
