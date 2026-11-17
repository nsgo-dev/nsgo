package keytab

import (
	"testing"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestTabWalksNativeFocusStops(t *testing.T) {
	presses := 0
	a := nsgotest.Start(t, func(*core.Scope) core.Node {
		row := core.Style{Width: core.Pt(160), Height: core.Pt(30)}
		return core.View{Children: []core.Node{
			core.Input{TestID: "first", Style: row},
			core.View{Style: row},
			core.View{TestID: "button", Style: row, OnPress: func(core.PressEvent) { presses++ }},
			core.View{Style: row, Disabled: func() bool { return true }, OnPress: func(core.PressEvent) {}},
			core.View{TestID: "explicit", Style: row, Focusable: func() bool { return true }},
			core.Input{TestID: "last", Style: row},
		}}
	}, nsgotest.Size(240, 240), nsgotest.Environment(driver.Environment{OS: driver.OSWeb}))
	a.Focus("first")
	for _, id := range []string{"button", "explicit", "last"} {
		a.Key("Tab")
		if !a.Find(id).Focused {
			t.Fatalf("Tab did not focus %s\n%s", id, a.Dump())
		}
	}
	for _, id := range []string{"explicit", "button", "first"} {
		a.Key("shift+Tab")
		if !a.Find(id).Focused {
			t.Fatalf("Shift+Tab did not focus %s\n%s", id, a.Dump())
		}
	}
	a.Key("Tab")
	a.Key("Space")
	a.Key("Enter")
	if presses != 2 {
		t.Fatalf("keyboard activations = %d, want 2", presses)
	}
}

func TestTabMovesBetweenFields(t *testing.T) {
	a := nsgotest.Start(t, func(*core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.Input{TestID: "first"}, core.View{}, core.Input{TestID: "second"},
		}}
	}, nsgotest.Environment(driver.Environment{OS: driver.OSWeb}))
	a.Focus("first")
	a.Key("Tab")
	if !a.Find("second").Focused {
		t.Fatal("Tab did not move the focus")
	}
	a.Key("shift+Tab")
	if !a.Find("first").Focused {
		t.Fatal("Shift+Tab did not return to the first field")
	}
}
