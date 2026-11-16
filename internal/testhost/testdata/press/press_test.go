package press_test

import (
	"testing"

	_ "github.com/nsgo-dev/nsgo"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

// TestPressModifiersReachTheApp uses the same press steps headless and
// through the targets' native input, checking every edge and the next tap.
func TestPressModifiersReachTheApp(t *testing.T) {
	if nsgotest.Target() == "watchos_simulator" {
		t.Skip("watchOS has no keyboard modifier input")
	}
	type received struct {
		phase string
		mods  core.Modifiers
	}
	var events []received
	var menus []core.Modifiers
	note := func(phase string) func(core.PressEvent) {
		return func(e core.PressEvent) { events = append(events, received{phase, e.Modifiers}) }
	}
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(20)}, Children: []core.Node{
			core.View{TestID: "target", Style: core.Style{Width: core.Pt(180), Height: core.Pt(100)},
				OnPressDown: note("down"), OnPressUp: note("up"), OnPress: note("press"),
				OnContextMenu: func(e core.ContextMenuEvent) { menus = append(menus, e.Modifiers) },
				Children:      []core.Node{core.Text{Content: "Click here", TestID: "label"}}},
		}}
	}, nsgotest.Size(300, 240))
	check := func(mods core.Modifiers) {
		t.Helper()
		phases := []string{"down", "up", "press"}
		// AppKit and Chromium on macOS also report a context menu for
		// Control-click; Chromium replaces the click with that event.
		if (nsgotest.Target() == "web" || nsgotest.Target() == "macos") && mods.Control && len(menus) == 1 && menus[0] == mods {
			if nsgotest.Target() == "web" {
				phases = phases[:2]
			} else {
				phases = nil
			}
		} else if len(menus) != 0 {
			t.Fatalf("unexpected context menus with %v: %v", mods, menus)
		}
		menus = nil
		if len(events) != len(phases) {
			t.Fatalf("press with %v reported %v, want %v", mods, events, phases)
		}
		for i, phase := range phases {
			if events[i].phase != phase || events[i].mods != mods {
				t.Fatalf("event %d = %v, want %s with %v", i, events[i], phase, mods)
			}
		}
		events = nil
	}
	for _, mods := range []core.Modifiers{{Shift: true}, {Alt: true}, {Platform: true}, {Control: true},
		{Shift: true, Alt: true, Control: true, Platform: true}} {
		app.PressWith("id:label", mods) // a text resolves to its pressable ancestor
		check(mods)
		app.Press("target")
		check(core.Modifiers{})
		app.PressAtWith("target", core.Point{X: 70, Y: 60}, mods)
		check(mods)
		app.PressAt("target", core.Point{X: 70, Y: 60})
		check(core.Modifiers{})
	}
}
