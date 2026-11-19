package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func start(t *testing.T) *nsgotest.App {
	t.Helper()
	return nsgotest.Start(t, Root, nsgotest.Size(390, 760))
}

// open pushes the demo named name from the index.
func open(t *testing.T, app *nsgotest.App, name string) {
	t.Helper()
	app.Press("id:nav-" + name)
	app.Find("id:screen-" + name)
}

func TestEveryDemoOpensAndGoesBack(t *testing.T) {
	app := start(t)
	if app.Exists("id:back") {
		t.Fatal("the index shows back")
	}
	for _, g := range groups {
		app.Find("id:group-" + g.name)
		for _, d := range g.demos {
			open(t, app, d.name)
			if len(app.Query("id:demo-scroll")) == 0 {
				t.Errorf("%s shows no cards", d.name)
			}
			app.Press("id:back")
			app.WaitForGone("id:screen-"+d.name, 0)
			app.Find("id:home")
		}
	}
}

func TestButtonsCount(t *testing.T) {
	app := start(t)
	open(t, app, "Button")
	if app.Enabled("id:reset") {
		t.Error("reset is enabled before any press")
	}
	app.Press("id:press")
	if got := app.Find("id:count").Text; got != "Pressed once" {
		t.Errorf("count = %q", got)
	}
	app.Press("id:press")
	if got := app.Find("id:count").Text; got != "Pressed 2 times" {
		t.Errorf("count = %q", got)
	}
	app.Press("id:reset")
	if got := app.Find("id:count").Text; got != "Not pressed yet" {
		t.Errorf("count after reset = %q", got)
	}
}

func TestInputsEcho(t *testing.T) {
	app := start(t)
	open(t, app, "Input")
	app.Type("id:name", "Ada")
	if got := app.Find("id:greeting").Text; got != "Hello, Ada!" {
		t.Errorf("greeting = %q", got)
	}
	app.Type("id:secret", "hunter2")
	if got := app.Find("id:secret-length").Text; got != "7 characters" {
		t.Errorf("secret length = %q", got)
	}
}

func TestTogglesChange(t *testing.T) {
	app := start(t)
	open(t, app, "Toggles")
	app.Press("id:switch")
	if got := app.Find("id:switch-state").Text; got != "Wi-Fi off" {
		t.Errorf("switch = %q", got)
	}
	app.Press("id:check-Banana")
	app.Press("id:check-Cherry")
	if got := app.Find("id:checked").Text; got != "3 of 3 picked" {
		t.Errorf("checked = %q", got)
	}
	app.Press("id:check-Apple")
	if got := app.Find("id:checked").Text; got != "2 of 3 picked" {
		t.Errorf("checked after unchecking = %q", got)
	}
}

// TestProgressFillsItsPercent checks that the slider's 40 fills 40% of the progress bar's track.
func TestProgressFillsItsPercent(t *testing.T) {
	app := start(t)
	open(t, app, "Slider")
	track := app.Find("id:level-progress").Children[0]
	indicator := track.Children[0]
	if got, want := indicator.Frame.Size.Width, track.Frame.Size.Width*0.4; got < want-1 || got > want+1 {
		t.Errorf("indicator is %v wide in a track %v wide, want %v", got, track.Frame.Size.Width, want)
	}
}

func TestTabsSwitchTheView(t *testing.T) {
	app := start(t)
	open(t, app, "Tabs")
	app.Press("id:tab-Month")
	if got := app.Find("id:tab-content").Text; got != "Month's view" {
		t.Errorf("tab content = %q", got)
	}
}

func TestListSelects(t *testing.T) {
	app := start(t)
	open(t, app, "List")
	app.Press(`text:"Row 3"`)
	if got := app.Find("id:picked").Text; got != "Row 3 selected" {
		t.Errorf("picked = %q", got)
	}
}

func TestNavStackPushesAndPops(t *testing.T) {
	app := start(t)
	open(t, app, "NavStack")
	app.Press("id:screen-NavStack >> id:push")
	app.Find("id:page-1")
	app.Press("id:page-1 >> id:push")
	app.Find("id:page-2")
	app.Press("id:back")
	app.WaitForGone("id:page-2", 0)
	app.Find("id:page-1")
	app.Press("id:root")
	app.WaitForGone("id:page-1", 0)
	app.Find("id:home")
}

func TestServicesAnswer(t *testing.T) {
	app := start(t)
	open(t, app, "Services")
	app.Press("id:alert")
	alerts := app.Alerts()
	if len(alerts) != 1 || alerts[0].Title != "Delete the draft?" {
		t.Fatalf("alerts = %+v", alerts)
	}
	app.Answer(1)
	if got := app.Find("id:answer").Text; got != "Deleted" {
		t.Errorf("answer = %q", got)
	}
	app.Press("id:copy")
	if got := app.Find("id:copied").Text; got != "Copied" {
		t.Errorf("copied = %q", got)
	}
	if got := app.Find("id:visits").Text; got != "Opened once" {
		t.Errorf("visits = %q", got)
	}
}
