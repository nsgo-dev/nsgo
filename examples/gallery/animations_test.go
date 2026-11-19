package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestExitsLeaveWithTheirMotion hides the Exits card, which stays while
// its exit plays and goes once it ends.
func TestExitsLeaveWithTheirMotion(t *testing.T) {
	headless(t)
	app := startDesktop(t, nsgotest.ManualFrames())
	open(app, "Animations")
	app.Advance(time.Second)
	app.Press("exit-toggle")
	app.Advance(50 * time.Millisecond)
	if !app.Exists("exit-card") {
		t.Fatal("the card went before its exit played")
	}
	if o := app.Animated("exit-card").Opacity; o <= 0 || o >= 1 {
		t.Fatalf("leaving, the card shows at opacity %v", o)
	}
	app.Advance(time.Second)
	if app.Exists("exit-card") {
		t.Fatal("the card stayed after its exit")
	}
	app.Press("exit-toggle")
	app.Advance(time.Second)
	if !app.Exists("exit-card") || app.Animated("exit-card").Opacity != 1 {
		t.Fatal("the card did not come back")
	}
}

// TestSharedElementsOpenATile opens a tile into its panel and back, on
// any target, the panel growing from the tile.
func TestSharedElementsOpenATile(t *testing.T) {
	var app *nsgotest.App
	if target := nsgotest.Target(); target == "ios_simulator" || target == "android" {
		app = startPhone(t)
		openPhone(app, "Animations")
	} else {
		app = startDesktop(t)
		open(app, "Animations")
	}
	app.Press("tile-2")
	if h := app.Find("tile-2").Frame.Size.Height; h != 120 {
		t.Fatalf("the opened tile is %v tall, want the panel's 120", h)
	}
	for _, id := range []string{"tile-0", "tile-1", "tile-3"} {
		if h := app.Find(id).Frame.Size.Height; h != 48 {
			t.Fatalf("%s is %v tall beside the panel, want 48", id, h)
		}
	}
	app.Screenshot("shared-element-open")
	app.Press("tile-2")
	if h := app.Find("tile-2").Frame.Size.Height; h != 48 {
		t.Fatalf("the closed tile is %v tall, want 48", h)
	}
}

// TestResistingDragsSlowPastTheEnds drags the knob past its track's end:
// it shows resisted short of the drag, then springs back to the end.
func TestResistingDragsSlowPastTheEnds(t *testing.T) {
	headless(t)
	app := startDesktop(t, nsgotest.ManualFrames())
	open(app, "Animations")
	app.Pan("resist-knob", core.Point{X: 40}, core.Point{X: 300})
	if x := app.Animated("resist-knob").Transform.TranslateX; x <= 80 || x >= 300 {
		t.Fatalf("dragged 300 points, the knob shows at %v, want resisted past 80", x)
	}
	app.Advance(2 * time.Second)
	if x := app.Animated("resist-knob").Transform.TranslateX; x < 79.5 || x > 80.5 {
		t.Fatalf("let go, the knob rests at %v, want the track's end at 80", x)
	}
}

// TestDemoSwitchFades fades a demo selected in the desktop layout in, the one it starts with shown at once; a demo selected before the last finished fading goes at once, and reduced motion shows the next at once.
func TestDemoSwitchFades(t *testing.T) {
	headless(t)
	app := startDesktop(t, nsgotest.ManualFrames())
	if o := app.Animated("id:demo-body").Opacity; o != 1 {
		t.Fatalf("the first demo shows at opacity %v", o)
	}
	open(app, "Checkbox")
	app.Advance(100 * time.Millisecond)
	if o := app.Animated("id:demo-Checkbox >> id:demo-body").Opacity; o <= 0 || o >= 1 {
		t.Fatalf("fading in, the demo shows at opacity %v", o)
	}
	app.Advance(time.Second)
	if o := app.Animated("id:demo-Checkbox >> id:demo-body").Opacity; o != 1 {
		t.Fatalf("faded in, the demo shows at opacity %v", o)
	}
	open(app, "Slider")
	app.Advance(50 * time.Millisecond)
	open(app, "Input")
	if n := len(app.QueryMounted("id:demo-body")); n != 1 {
		t.Fatalf("%d demos are mounted after a quick switch, want 1", n)
	}
	if o := app.Animated("id:demo-Input >> id:demo-body").Opacity; o >= 1 {
		t.Fatalf("a quick switch shows the demo at opacity %v, not fading in", o)
	}

	reduced := desktopEnv
	reduced.ReduceMotion = true
	app = startDesktop(t, nsgotest.ManualFrames(), nsgotest.Environment(reduced))
	open(app, "Checkbox")
	if o := app.Animated("id:demo-Checkbox >> id:demo-body").Opacity; o != 1 {
		t.Fatalf("under reduced motion, the demo shows at opacity %v", o)
	}
}

// TestTabSwitchFades fades a tab of the phone layout in each time it is chosen, the one it starts with shown at once.
func TestTabSwitchFades(t *testing.T) {
	headless(t)
	app := nsgotest.Start(t, Root, nsgotest.ManualFrames(), nsgotest.Size(390, 844), nsgotest.Environment(core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}))
	first, second := "id:tab-content-"+tabs[0].id, "id:tab-content-"+tabs[1].id
	if o := app.Animated(first).Opacity; o != 1 {
		t.Fatalf("the first tab shows at opacity %v", o)
	}
	for range 2 {
		openTab(app, 1)
		if o := app.Animated(second).Opacity; o >= 1 {
			t.Fatalf("chosen, the tab shows at opacity %v, not fading in", o)
		}
		app.Advance(time.Second)
		if o := app.Animated(second).Opacity; o != 1 {
			t.Fatalf("faded in, the tab shows at opacity %v", o)
		}
		openTab(app, 0)
		if o := app.Animated(first).Opacity; o >= 1 {
			t.Fatalf("chosen again, the first tab shows at opacity %v, not fading in", o)
		}
		app.Advance(time.Second)
		if o := app.Animated(first).Opacity; o != 1 {
			t.Fatalf("faded in again, the first tab shows at opacity %v", o)
		}
	}

	// The settings' shortcut builds the tabs' page again on the settings, so the settings show at once and the first tab, chosen after, fades in.
	app.Key(settingsKey)
	app.Advance(time.Second)
	if o := app.Animated("id:tab-content-settings").Opacity; o != 1 {
		t.Fatalf("opened by its shortcut, the settings show at opacity %v", o)
	}
	openTab(app, 0)
	if o := app.Animated(first).Opacity; o >= 1 {
		t.Fatalf("after the settings' shortcut, the first tab shows at opacity %v, not fading in", o)
	}
}
