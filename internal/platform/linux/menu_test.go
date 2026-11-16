//go:build linux && !android

package linux

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// TestApplicationMenuBuildsAndAnswersItsShortcut shows the app menu as a
// GTK menu bar and runs the item a native key press matches.
func TestApplicationMenuBuildsAndAnswersItsShortcut(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-menu-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	app := func(cx *core.Scope) core.Node {
		bumped := core.NewSignal(cx, 0)
		cx.SetApplicationMenu([]core.MenuItem{
			{Label: "File", Items: []core.MenuItem{
				{Label: "Bump", Shortcut: "mod+b", OnSelect: func() { bumped.Update(func(n int) int { return n + 1 }) }},
				{Separator: true},
				{Label: "Quit", Role: driver.MenuQuit},
			}},
		})
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d", title, bumped.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100)}}
	}
	w := startTestWindow(t, engine.App(app), Options{Title: title, Width: 200, Height: 100})
	waitForTitle(t, w, title+"|0")
	w.onLoop(t, func() {
		if w.menuBar == 0 {
			t.Error("the application menu has no GTK menu bar")
		}
		if gtkWidgetGetParent(w.menuBar) != w.box {
			t.Error("the menu bar is not above the surface")
		}
	})
	w.onLoop(t, func() { keyPressedCallback(0, 'b', 0, gdkControl, w.handle) })
	waitForTitle(t, w, title+"|1")

	// An app rebuilds its menu whenever a checkable item changes. The
	// rebuilds must not run out of purego callbacks or GTK actions.
	w.onLoop(t, func() {
		entries := w.chrome.menu
		for range 250 {
			w.setMenu(entries)
		}
		if w.menuBar == 0 {
			t.Error("the menu bar went away after rebuilding")
		}
	})
}

func TestMenuYieldFollowsNativeTextFocus(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	const title = "nsgo-menu-yield"
	app := func(cx *core.Scope) core.Node {
		menu, binding, focus := core.NewSignal(cx, 0), core.NewSignal(cx, 0), core.NewSignal(cx, false)
		cx.SetApplicationMenu([]core.MenuItem{{Label: "Actions", Items: []core.MenuItem{
			{Label: "Refresh", Shortcut: "mod+r", When: core.MenuNotEditing, OnSelect: func() { menu.Update(func(n int) int { return n + 1 }) }},
		}}})
		cx.OnKeyWith("mod+r", core.KeyOptions{YieldsToMenu: true}, func() { binding.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d|%d|%v", title, menu.Get(), binding.Get(), focus.Get())) })
		return core.Input{OnFocus: func() { focus.Set(true) }, OnBlur: func() { focus.Set(false) },
			Style: core.Style{Width: driver.Pt(180), Height: driver.Pt(30)}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 240, Height: 120})
	waitForTitle(t, w, title+"|0|0|false")
	press := func() {
		w.onLoop(t, func() {
			keyPressedCallback(0, 'r', 27, gdkControl, w.handle)
			keyReleasedCallback(0, 'r', 27, gdkControl, w.handle)
		})
	}
	press()
	waitForTitle(t, w, title+"|1|0|false")
	w.onLoop(t, func() {
		for _, v := range w.entries {
			if v.kind == driver.KindInput {
				w.focusEntry(v)
				break
			}
		}
	})
	waitForTitle(t, w, title+"|1|0|true")
	press()
	waitForTitle(t, w, title+"|1|1|true")
	w.onLoop(t, func() { gtkWindowSetFocus(w.win, 0) })
	waitForTitle(t, w, title+"|1|1|false")
	press()
	waitForTitle(t, w, title+"|2|1|false")
}

func TestMenuYieldFollowsNativeWebFocus(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	// WebKit retains its initialization thread; isolate it from tests whose GTK windows use other threads.
	if os.Getenv("NSGO_TEST_MENU_WEB") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_MENU_WEB="+t.Name())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	if err := openWebKit(); err != nil {
		t.Skip(err)
	}
	const title = "nsgo-menu-web-yield"
	app := func(cx *core.Scope) core.Node {
		menu, binding, loaded := core.NewSignal(cx, 0), core.NewSignal(cx, 0), core.NewSignal(cx, false)
		cx.SetApplicationMenu([]core.MenuItem{{Label: "Actions", Items: []core.MenuItem{
			{Label: "Refresh", Shortcut: "mod+r", When: core.MenuNotEditing, OnSelect: func() { menu.Update(func(n int) int { return n + 1 }) }},
		}}})
		cx.OnKeyWith("mod+r", core.KeyOptions{YieldsToMenu: true}, func() { binding.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d|%d|%v", title, menu.Get(), binding.Get(), loaded.Get())) })
		return core.WebView{Source: core.WebHTML("<!doctype html><input>", ""),
			OnLoad: func(core.WebNavigation) { loaded.Set(true) }, Style: core.Style{Width: driver.Pt(240), Height: driver.Pt(120)}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 280, Height: 160})
	waitForTitle(t, w, title+"|0|0|true")
	setFocus := func(focused bool) {
		t.Helper()
		w.onLoop(t, func() {
			if !focused {
				gtkWindowSetFocus(w.win, 0)
				return
			}
			for _, v := range w.views {
				if v != nil && v.web != nil && v.web.widget != 0 {
					gtkWidgetGrabFocus(v.web.widget)
					return
				}
			}
			t.Error("no web view")
		})
		deadline := time.Now().Add(5 * time.Second)
		for {
			ready := false
			w.onLoop(t, func() {
				bound := slices.ContainsFunc(w.keys, func(k driver.KeyBinding) bool { return k.Key == "mod+r" })
				ready = bound == focused && (w.focusedWeb() != nil) == focused
			})
			if ready {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("keymap did not follow web focus %v", focused)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	press := func() {
		w.onLoop(t, func() {
			keyPressedCallback(0, 'r', 27, gdkControl, w.handle)
			keyReleasedCallback(0, 'r', 27, gdkControl, w.handle)
		})
	}
	setFocus(false)
	press()
	waitForTitle(t, w, title+"|1|0|true")
	setFocus(true)
	press()
	waitForTitle(t, w, title+"|1|1|true")
	setFocus(false)
	press()
	waitForTitle(t, w, title+"|2|1|true")
}

// TestRadioMenuItemsActivate builds radio items, whose actions GTK activates with the item's target, so each takes a string parameter and is string-stated, the checked one's state its target; choosing the other runs it. The roles only macOS has, and the Help holder with no items, are left out.
func TestRadioMenuItemsActivate(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	var (
		activate  func(group uintptr, name *byte, parameter uintptr)
		getName   func(action uintptr) string
		paramType func(action uintptr) uintptr
		getState  func(action uintptr) uintptr
		getString func(v uintptr, length uintptr) string
		unref     func(v uintptr)
	)
	title := "nsgo-menu-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	app := func(cx *core.Scope) core.Node {
		library := core.NewSignal(cx, "Movies")
		cx.Effect(func() {
			lib := library.Get()
			radio := func(name string) core.MenuItem {
				return core.MenuItem{Label: name, Radio: true, Checked: lib == name, OnSelect: func() { library.Set(name) }}
			}
			cx.SetApplicationMenu([]core.MenuItem{
				{Label: "Library", Items: []core.MenuItem{radio("Movies"), radio("Courses"), {Separator: true},
					{Role: core.MenuServices}, {Role: core.MenuStartSpeaking}, {Role: core.MenuBringAllToFront}}},
				{Label: "Help", Role: core.MenuHelpMenu},
			})
			cx.SetTitle(title + "|" + lib)
		})
		return core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100)}}
	}
	w := startTestWindow(t, engine.App(app), Options{Title: title, Width: 200, Height: 100})
	waitForTitle(t, w, title+"|Movies")
	// The libraries are loaded once the window is.
	purego.RegisterLibFunc(&activate, gioLib, "g_action_group_activate_action")
	purego.RegisterLibFunc(&getName, gioLib, "g_action_get_name")
	purego.RegisterLibFunc(&paramType, gioLib, "g_action_get_parameter_type")
	purego.RegisterLibFunc(&getState, gioLib, "g_action_get_state")
	purego.RegisterLibFunc(&getString, glibLib, "g_variant_get_string")
	purego.RegisterLibFunc(&unref, glibLib, "g_variant_unref")
	var courses string
	w.onLoop(t, func() {
		if len(w.actionRefs) != 2 {
			t.Errorf("the menu has %d actions, want the two radios'", len(w.actionRefs))
		}
		for action := range w.actionRefs {
			name := getName(action)
			if paramType(action) == 0 {
				t.Errorf("%s takes no parameter, so GTK cannot activate it with its target", name)
			}
			state := getState(action)
			if state == 0 {
				t.Errorf("%s has no state", name)
				continue
			}
			checked := getString(state, 0) == radioTarget
			unref(state)
			if !checked {
				courses = name
			}
		}
	})
	if courses == "" {
		t.Fatal("no radio is unchecked")
	}
	w.onLoop(t, func() { activate(w.actionGroup, cstr(courses), gVariantNewString(cstr(radioTarget))) })
	waitForTitle(t, w, title+"|Courses")
}
