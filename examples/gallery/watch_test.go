package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

// startWatch runs root on an Apple Watch.
func startWatch(t *testing.T, root func(cx *core.Scope) core.Node, opts ...nsgotest.Option) *nsgotest.App {
	if nsgotest.Target() != "watchos_simulator" {
		t.Skip("needs an Apple Watch: nsgo test -target watchos_simulator")
	}
	return nsgotest.Start(t, root, opts...)
}

// TestWatchPressAndScroll presses a button, scrolls a list longer than
// the screen to its end and presses the last row, which showed only once
// scrolled to.
func TestWatchPressAndScroll(t *testing.T) {
	count := 0
	var pressed string
	app := startWatch(t, func(cx *core.Scope) core.Node {
		n := core.NewSignal(cx, 0)
		rows := []core.Node{ui.Button{Text: "Add", TestID: "add", OnPress: func() {
			count++
			n.Set(count)
		}}, core.Text{TestID: "count", ContentFunc: func() string { return fmt.Sprint(n.Get()) }}}
		for i := range 40 {
			id := fmt.Sprintf("row-%d", i)
			rows = append(rows, ui.Button{Text: id, TestID: id, OnPress: func() { pressed = id }})
		}
		return core.Scroll{TestID: "list", Style: core.Style{FlexGrow: 1},
			Children: []core.Node{core.View{Style: core.Style{Padding: core.All(8), Gap: 8}, Children: rows}}}
	})
	app.Press("id:add")
	app.Press("id:add")
	if count != 2 || app.Find("id:count").Text != "2" {
		t.Fatalf("two presses counted %d, showing %q", count, app.Find("id:count").Text)
	}
	app.Screenshot("watch-list")
	app.Scroll("id:list", 5000)
	app.Press("id:row-39")
	if pressed != "row-39" {
		t.Fatalf("pressed %q, want the last row", pressed)
	}
}

// TestWatchInput types into a field the system's text input edits, and
// submits it.
func TestWatchInput(t *testing.T) {
	var submitted string
	app := startWatch(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(8)}, Children: []core.Node{
			ui.Input{TestID: "name", Placeholder: "Name", OnSubmit: func(s string) { submitted = s }},
		}}
	})
	app.Focus("id:name")
	app.Type("id:name", "Ann")
	app.Submit("id:name")
	if submitted != "Ann" {
		t.Fatalf("submitted %q, want Ann", submitted)
	}
}

// TestWatchAlert shows an alert and answers it.
func TestWatchAlert(t *testing.T) {
	answer := -2
	app := startWatch(t, func(cx *core.Scope) core.Node {
		return ui.Button{Text: "Ask", TestID: "ask", OnPress: func() {
			cx.ShowAlert(core.AlertConfig{Title: "Delete?", Actions: []core.AlertAction{
				{Label: "Cancel", Role: core.AlertCancel}, {Label: "Delete", Role: core.AlertDestructive}}},
				func(i int) { answer = i })
		}}
	})
	app.Press("id:ask")
	app.Answer(1)
	if answer != 1 {
		t.Fatalf("the alert answered %d, want 1", answer)
	}
}

// TestWatchGallery lists the demos a watch shows, opens each from the index and goes back to it, by the header's button and by the system's back, and answers the Dialog demo's alert.
func TestWatchGallery(t *testing.T) {
	app := startWatch(t, Root, nsgotest.Environment(core.Environment{Locale: "en-US"}))
	if app.Exists("id:nav-Stocks") || app.Exists("id:nav-WebView") {
		t.Fatal("the watch's index lists a demo made for wider screens")
	}
	app.Screenshot("watch/index")
	for i, d := range watchDemos() {
		app.Press("id:nav-" + d.name)
		app.WaitFor("id:screen-"+d.name, 5*time.Second)
		app.FastForward(time.Second)
		app.Screenshot("watch/" + strings.ToLower(d.name))
		if i%2 == 0 {
			app.Press("id:screen-" + d.name + " >> role:button Back")
		} else {
			app.Back()
		}
		app.WaitForGone("id:screen-"+d.name, 5*time.Second)
		if !app.Exists("id:index") {
			t.Fatalf("back from %s left the index", d.name)
		}
	}
	app.Press("id:nav-Dialog")
	app.Press("id:alert-open")
	app.Answer(1)
	if got := app.Find("id:alert-answer").Text; got != "Done" {
		t.Fatalf("the alert answered %q, want Done", got)
	}
}

// TestWatchLayout lays the gallery out as a watch does, headless: the index lists only the watch's demos, and a demo goes back to it.
func TestWatchLayout(t *testing.T) {
	if nsgotest.Target() != "" {
		t.Skip("headless: a target keeps its own idiom")
	}
	app := nsgotest.Start(t, Root, nsgotest.Size(162, 197), nsgotest.Environment(core.Environment{OS: core.OSIOS, Idiom: core.IdiomWatch, Locale: "en-US"}))
	for _, d := range allDemos() {
		if got, want := app.Exists("id:nav-"+d.name), watchPages[d.name] != nil; got != want {
			t.Errorf("the index lists %s: %v, want %v", d.name, got, want)
		}
	}
	app.Press("id:nav-Slider")
	app.WaitFor("id:screen-Slider", time.Second)
	app.Press("id:screen-Slider >> role:button Back")
	app.WaitForGone("id:screen-Slider", time.Second)
}
