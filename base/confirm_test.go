package base_test

import (
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Return runs a dialog's or a sheet's default action and closes it with ReasonSelect, once however often it is pressed as it goes, and again after it opens again; the page's own Return stays with the page once it has gone.
func TestReturnRunsTheDefaultActionOnce(t *testing.T) {
	for _, c := range []struct {
		name  string
		env   core.Environment
		sheet bool
	}{
		{"dialog", core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}, false},
		{"dialog-windows", core.Environment{OS: core.OSWindows, Idiom: core.IdiomDesktop}, false},
		{"sheet", core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone}, true},
		{"sheet-android", core.Environment{OS: core.OSAndroid, Idiom: core.IdiomPhone}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			var open *core.Signal[bool]
			var log []string
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				open = core.NewSignal(cx, true)
				cx.OnKey("enter", func() { log = append(log, "page") })
				change := func(o bool, r base.Reason) { log = append(log, r.String()); open.Set(o) }
				confirm := func() { log = append(log, "confirm") }
				content := []core.Node{core.Text{Content: "Save the changes?", TestID: "inside"}}
				if c.sheet {
					return base.Sheet{Open: open.Get, OnOpenChange: change, OnConfirm: confirm, Role: core.RoleAlertDialog, Label: "Save",
						AccessibilityDescription: "They are lost otherwise.", TestID: "layer", PopupStyle: core.Style{Height: core.Pt(200)}, Children: content}
				}
				// An ending part keeps it in the tree as it goes, with its Return.
				return base.Dialog{Open: open.Get, OnOpenChange: change, OnConfirm: confirm, Role: core.RoleAlertDialog, Label: "Save",
					AccessibilityDescription: "They are lost otherwise.", TestID: "layer", Children: content, Animation: core.ModalNone,
					PopupStyle:  core.Style{Transition: core.Transition{Duration: 200 * time.Millisecond, Properties: core.TransitionOpacity}},
					EndingStyle: core.Style{Opacity: core.Alpha(0)}}
			}, nsgotest.Environment(c.env))
			layer := app.Find("role:alertdialog")
			if layer.Props.Label != "Save" || layer.Props.Description != "They are lost otherwise." {
				t.Fatalf("layer accessibility = %+v", layer.Props)
			}
			app.Advance(time.Second)
			app.Key("Return")
			// A dialog's ending part keeps it in the tree as it goes, where Return runs nothing more; nsgotest runs a sheet's motion out at once.
			if !c.sheet {
				if !app.Exists("id:inside") {
					t.Fatal("the dialog left without its motion")
				}
				app.Key("Return")
			}
			if want := []string{"confirm", base.ReasonSelect.String()}; !slices.Equal(log, want) {
				t.Fatalf("Return as it goes: log = %q, want %q", log, want)
			}
			app.Advance(time.Second)
			app.WaitForGone("id:inside", time.Second)
			log = nil
			app.Key("Return")
			if want := []string{"page"}; !slices.Equal(log, want) {
				t.Fatalf("Return once it had gone: log = %q, want %q", log, want)
			}
			log = nil
			app.Post(func() { open.Set(true) })
			app.WaitFor("id:inside", time.Second)
			app.Key("Return")
			if want := []string{"confirm", base.ReasonSelect.String()}; !slices.Equal(log, want) {
				t.Fatalf("Return after opening again: log = %q, want %q", log, want)
			}
		})
	}
}

// Return stays with a focused text input in the dialog, and with a dialog over it that has no default action, until that one closes.
func TestReturnLeavesTheDefaultToWhatHasIt(t *testing.T) {
	var field, upper *core.Signal[bool]
	var log []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		field, upper = core.NewSignal(cx, true), core.NewSignal(cx, false)
		return base.Dialog{Open: func() bool { return true }, OnOpenChange: func(o bool, r base.Reason) { log = append(log, r.String()) },
			OnConfirm: func() { log = append(log, "confirm") }, Label: "Rename", Children: []core.Node{
				core.Show{When: field.Get, Then: func(*core.Scope) core.Node {
					return core.Input{AutoFocus: true, TestID: "name", Style: core.Style{Width: core.Pt(100), Height: core.Pt(20)}}
				}},
				base.Dialog{Open: upper.Get, OnOpenChange: func(o bool, _ base.Reason) { upper.Set(o) }, Label: "Help",
					Children: []core.Node{core.Text{Content: "Help", TestID: "help"}}},
			}}
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	if !app.Find("id:name").Focused {
		t.Fatal("the field did not take the focus")
	}
	app.Key("Return")
	if len(log) != 0 {
		t.Fatalf("Return in a focused field: log = %q", log)
	}
	app.Post(func() { field.Set(false); upper.Set(true) })
	app.WaitFor("id:help", time.Second)
	app.Key("Return")
	if len(log) != 0 {
		t.Fatalf("Return under another dialog: log = %q", log)
	}
	app.Key("Escape")
	app.Advance(time.Second)
	app.WaitForGone("id:help", time.Second)
	app.Key("Return")
	if want := []string{"confirm", base.ReasonSelect.String()}; !slices.Equal(log, want) {
		t.Fatalf("Return once the dialog over it closed: log = %q, want %q", log, want)
	}
}

// A default action that closes the dialog itself still reports the close with ReasonSelect.
func TestTheDefaultActionMayCloseTheDialogItself(t *testing.T) {
	var log []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		open := core.NewSignal(cx, true)
		return base.Dialog{Open: open.Get, OnOpenChange: func(o bool, r base.Reason) { log = append(log, r.String()); open.Set(o) },
			OnConfirm: func() { log = append(log, "confirm"); open.Set(false) }, Label: "Done",
			Children: []core.Node{core.Text{Content: "Done", TestID: "inside"}}}
	}, nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	app.Find("id:inside")
	app.Key("Return")
	if want := []string{"confirm", base.ReasonSelect.String()}; !slices.Equal(log, want) {
		t.Fatalf("log = %q, want %q", log, want)
	}
}
