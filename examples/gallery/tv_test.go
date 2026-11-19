package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

// startTV runs root on an Apple TV, whose remote moves a focus.
func startTV(t *testing.T, root func(cx *core.Scope) core.Node, opts ...nsgotest.Option) *nsgotest.App {
	if nsgotest.Target() != "tvos_simulator" {
		t.Skip("needs an Apple TV: nsgo test -target tvos_simulator")
	}
	return nsgotest.Start(t, root, opts...)
}

// focusedID is the test ID of the view with the focus, or "".
func focusedID(app *nsgotest.App, ids ...string) string {
	for _, id := range ids {
		if app.Find("id:" + id).Focused {
			return id
		}
	}
	return ""
}

// focusWithin reports whether v or a view inside it has the focus.
func focusWithin(v *nsgotest.View) bool {
	if v.Focused {
		return true
	}
	for _, c := range v.Children {
		if focusWithin(c) {
			return true
		}
	}
	return false
}

// TestTVRemote moves the focus between buttons with the remote's arrows
// and presses the focused one with its select button.
func TestTVRemote(t *testing.T) {
	var pressed []string
	app := startTV(t, func(cx *core.Scope) core.Node {
		button := func(id string) core.Node {
			return ui.Button{Text: id, TestID: id, OnPress: func() { pressed = append(pressed, id) }}
		}
		return core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter, Gap: 40},
			Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 40}, Children: []core.Node{button("a"), button("b")}},
				button("c"),
			}}
	})
	ids := []string{"a", "b", "c"}
	if got := focusedID(app, ids...); got != "a" {
		t.Fatalf("the focus starts on %q, want the first button", got)
	}
	app.Key("right")
	if got := focusedID(app, ids...); got != "b" {
		t.Fatalf("right moved the focus to %q, want b", got)
	}
	app.Key("down")
	if got := focusedID(app, ids...); got != "c" {
		t.Fatalf("down moved the focus to %q, want c", got)
	}
	app.Key("enter")
	if len(pressed) != 1 || pressed[0] != "c" {
		t.Fatalf("select pressed %v, want [c]", pressed)
	}
}

// TestTVModalFocus moves the focus into a dialog as it opens, keeps it
// there whichever way the remote moves it, and gives it back to the
// button that opened it as the remote's Menu button closes the dialog.
func TestTVModalFocus(t *testing.T) {
	app := startTV(t, func(cx *core.Scope) core.Node {
		open := core.NewSignal(cx, false)
		return core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter, Gap: 40},
			Children: []core.Node{
				ui.Button{Text: "Other", TestID: "other", OnPress: func() {}},
				ui.Button{Text: "Open", TestID: "open", OnPress: func() { open.Set(true) }},
				ui.Dialog{Open: open.Get, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Title: "Dialog", TestID: "dialog", Children: []core.Node{
					ui.Button{Text: "Inside", TestID: "inside", OnPress: func() {}},
				}},
			}}
	})
	app.Key("down")
	if got := focusedID(app, "other", "open"); got != "open" {
		t.Fatalf("down moved the focus to %q, want open", got)
	}
	app.Key("enter")
	if !focusWithin(app.Find("id:dialog")) {
		t.Fatal("the dialog opened without the focus")
	}
	for _, key := range []string{"up", "down", "left", "right"} {
		app.Key(key)
		if !focusWithin(app.Find("id:dialog")) {
			t.Fatalf("%s moved the focus out of the dialog", key)
		}
	}
	app.Key("escape")
	app.FastForward(time.Second)
	if app.Exists("id:inside") {
		t.Fatal("Menu left the dialog open")
	}
	if got := focusedID(app, "other", "open"); got != "open" {
		t.Fatalf("the focus went back to %q, want the button that opened the dialog", got)
	}
}

// TestTVGallery opens demos from the gallery's sidebar, moves the remote's focus down the sidebar and opens the next demo with its select button, pushes a page from a demo and goes back with the Menu button, and leaves out the demos a TV cannot use.
func TestTVGallery(t *testing.T) {
	app := startTV(t, Root, nsgotest.Environment(core.Environment{Locale: "en-US"}))
	for _, name := range []string{"Gestures", "WebView", "Editor", "Dock"} {
		if app.Exists("id:nav-" + name) {
			t.Fatalf("the TV's sidebar lists %s", name)
		}
	}
	app.WaitFor("id:demo-Stocks", 5*time.Second)
	app.Screenshot("tv/stocks")
	for _, name := range []string{"Button", "Checkbox", "Progress", "Tabs"} {
		app.Press("id:nav-" + name)
		app.WaitFor("id:demo-"+name, 5*time.Second)
		app.FastForward(time.Second)
		app.Screenshot("tv/" + strings.ToLower(name))
	}
	var rows []string
	for _, d := range tvDemos() {
		rows = append(rows, "nav-"+d.name)
	}
	at := slices.Index(rows, focusedID(app, rows...))
	if at < 0 || at == len(rows)-1 {
		t.Fatalf("the focus is on %q, want a row of the sidebar above its last", focusedID(app, rows...))
	}
	app.Key("down")
	next := rows[at+1]
	if got := focusedID(app, rows...); got != next {
		t.Fatalf("down moved the focus to %q, want %s", got, next)
	}
	app.Key("enter")
	app.WaitFor("id:demo-"+strings.TrimPrefix(next, "nav-"), 5*time.Second)
	app.Press("id:nav-NavStack")
	app.WaitFor("id:demo-NavStack", 5*time.Second)
	app.Press("text:Large title")
	app.WaitFor("text:Row 20", 5*time.Second)
	app.FastForward(time.Second)
	app.Screenshot("tv/pushed")
	app.Key("escape")
	app.WaitForGone("text:Row 20", 5*time.Second)
	app.WaitFor("id:demo-NavStack", 5*time.Second)
}

// TestTVLayout lays the gallery out as a TV does, headless: the sidebar leaves out the demos a TV cannot use, and opens the others beside it.
func TestTVLayout(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(1920, 1080), nsgotest.Environment(core.Environment{OS: core.OSIOS, Idiom: core.IdiomTV, Locale: "en-US"}))
	for _, d := range allDemos() {
		if got := app.Exists("id:nav-" + d.name); got == tvLeftOut[d.name] {
			t.Errorf("the sidebar lists %s: %v", d.name, got)
		}
	}
	app.Press("id:nav-Checkbox")
	app.WaitFor("id:demo-Checkbox", time.Second)
	if got := app.Find("id:title").Text; got != "Checkbox" {
		t.Fatalf("the title says %q, want Checkbox", got)
	}
}
