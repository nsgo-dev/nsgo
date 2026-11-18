//go:build android && nsgotest

package android

import (
	"sync/atomic"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestHardwareKeysBeforeFocus(t *testing.T) {
	var pressed atomic.Int32
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.OnKey("space", func() { pressed.Add(1) })
		return core.Text{Content: "Space plays and pauses"}
	})
	for n := range 2 {
		app.Key("space")
		if got := pressed.Load(); got != int32(n+1) {
			t.Fatalf("before any control took focus, Space fired %d times; want %d", got, n+1)
		}
	}
}

func TestHardwareKeysLeaveInputsUnfocused(t *testing.T) {
	var pressed, focused atomic.Int32
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.OnKey("space", func() { pressed.Add(1) })
		return core.View{Children: []core.Node{
			core.Input{TestID: "first", Style: core.Style{Width: core.Pt(240), Height: core.Pt(44)}, OnFocus: func() { focused.Add(1) }},
			core.Input{TestID: "second", Style: core.Style{Width: core.Pt(240), Height: core.Pt(44)}, OnFocus: func() { focused.Add(1) }},
		}}
	})
	if app.Find("id:first").Focused || app.Find("id:second").Focused || focused.Load() != 0 {
		t.Fatal("an input without AutoFocus took the startup focus")
	}
	app.Key("space")
	if pressed.Load() != 1 || app.Find("id:first").Focused || app.Find("id:second").Focused || focused.Load() != 0 {
		t.Fatalf("Space with no field focused: binding=%d field focuses=%d", pressed.Load(), focused.Load())
	}
}

func TestHardwareKeysResumeAfterBlur(t *testing.T) {
	hardwareKeysResume(t, "blur")
}

func TestHardwareKeysResumeAfterDismiss(t *testing.T) {
	hardwareKeysResume(t, "dismiss")
}

func TestHardwareKeysResumeAfterSubmit(t *testing.T) {
	hardwareKeysResume(t, "submit")
}

func hardwareKeysResume(t *testing.T, leave string) {
	t.Helper()
	var pressed, blurred, secondFocus, submits atomic.Int32
	var text atomic.Value
	text.Store("")
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var field core.Ref
		cx.OnKey("space", func() { pressed.Add(1) })
		cx.OnKeyWith("mod+b", core.KeyOptions{When: core.KeyAlways}, func() {
			if leave == "dismiss" {
				cx.DismissKeyboard()
			} else {
				field.Blur()
			}
		})
		return core.View{Children: []core.Node{
			core.Input{TestID: "first", Ref: &field, AutoFocus: true,
				Style:    core.Style{Width: core.Pt(240), Height: core.Pt(44)},
				OnChange: func(value string) { text.Store(value) }, OnBlur: func() { blurred.Add(1) },
				OnSubmit: func(string) { submits.Add(1) }},
			core.Input{TestID: "second", Style: core.Style{Width: core.Pt(240), Height: core.Pt(44)}, OnFocus: func() { secondFocus.Add(1) }},
		}}
	})
	if !app.Find("id:first").Focused {
		t.Fatal("AutoFocus did not reach the field")
	}
	app.Key("space")
	if pressed.Load() != 0 || text.Load() != " " {
		t.Fatalf("focused field: binding=%d text=%q", pressed.Load(), text.Load())
	}
	if leave == "submit" {
		app.Key("enter")
		if submits.Load() != 1 {
			t.Fatalf("Return submitted %d times; want once", submits.Load())
		}
	} else {
		app.Key("mod+b")
	}
	if app.Find("id:first").Focused || app.Find("id:second").Focused || blurred.Load() != 1 || secondFocus.Load() != 0 {
		t.Fatalf("blur refocused a field: blurs=%d second focuses=%d", blurred.Load(), secondFocus.Load())
	}
	app.Key("space")
	if pressed.Load() != 1 || text.Load() != " " {
		t.Fatalf("after blur: binding=%d text=%q", pressed.Load(), text.Load())
	}
}

// A focused field that leaves the tree, as a search field closed by a shortcut does, gives the focus back to the host rather than to the window's next field.
func TestHardwareKeysResumeAfterFocusedInputGoes(t *testing.T) {
	var pressed, secondFocus atomic.Int32
	var text atomic.Value
	text.Store("")
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		searching := core.NewSignal(cx, true)
		cx.OnKey("space", func() { pressed.Add(1) })
		cx.OnKeyWith("mod+b", core.KeyOptions{When: core.KeyAlways}, func() { searching.Set(false) })
		return core.View{Children: []core.Node{
			core.Show{When: searching.Get, Then: func(*core.Scope) core.Node {
				return core.Input{TestID: "search", AutoFocus: true, Style: core.Style{Width: core.Pt(240), Height: core.Pt(44)},
					OnChange: func(value string) { text.Store(value) }}
			}},
			core.Input{TestID: "other", Style: core.Style{Width: core.Pt(240), Height: core.Pt(44)}, OnFocus: func() { secondFocus.Add(1) }},
		}}
	})
	if !app.Find("id:search").Focused {
		t.Fatal("AutoFocus did not reach the search field")
	}
	app.Key("space")
	if pressed.Load() != 0 || text.Load() != " " {
		t.Fatalf("focused field: binding=%d text=%q", pressed.Load(), text.Load())
	}
	app.Key("mod+b")
	if app.Exists("id:search") {
		t.Fatal("the shortcut did not remove the search field")
	}
	if app.Find("id:other").Focused || secondFocus.Load() != 0 {
		t.Fatalf("removing the focused field focused another one %d times", secondFocus.Load())
	}
	app.Key("space")
	if pressed.Load() != 1 {
		t.Fatalf("after the focused field went, Space fired %d times; want once", pressed.Load())
	}
}

func TestHardwareKeysCanFocusInput(t *testing.T) {
	var text atomic.Value
	text.Store("")
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var field core.Ref
		cx.OnKey("mod+f", field.Focus)
		return core.Input{TestID: "field", Ref: &field, Style: core.Style{Width: core.Pt(240), Height: core.Pt(44)},
			OnChange: func(value string) { text.Store(value) }}
	})
	if app.Find("id:field").Focused {
		t.Fatal("the field took focus without AutoFocus")
	}
	app.Key("mod+f")
	if !app.Find("id:field").Focused {
		t.Fatal("the startup shortcut did not focus the field")
	}
	app.Key("space")
	if text.Load() != " " {
		t.Fatalf("explicitly focused field text = %q; want a space", text.Load())
	}
}
