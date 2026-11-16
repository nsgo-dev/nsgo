//go:build linux && !android

package linux

import (
	"fmt"
	"strings"
	"testing"

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
