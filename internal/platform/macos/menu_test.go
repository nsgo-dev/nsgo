//go:build darwin && !ios

package macos

import (
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestMenuSyncsInPlace checks that a menu set again keeps the items the
// menu bar shows, which setMainMenu: would redraw, and takes the new
// one's states, actions and items.
func TestMenuSyncsInPlace(t *testing.T) {
	onMain(func() { menuSyncsInPlace(t) })
}

func menuSyncsInPlace(t *testing.T) {
	loadAppKit()
	s := &surface{}
	s.ensureMenuTarget()
	defer delete(menuTargets, s.menuTarget)
	view := func(checked bool, first uint32, more ...driver.MenuEntry) []driver.MenuEntry {
		items := append([]driver.MenuEntry{
			{Label: "Sidebar", Checkable: true, Checked: checked, ID: first},
			{Separator: true},
			{Label: "Button", ID: first + 1},
		}, more...)
		return []driver.MenuEntry{{Label: "View", Items: items}}
	}
	bar := s.submenu("", view(true, 1))
	defer bar.Send(selRelease)
	holder := bar.Send(selItemAtIndex, 0)
	sub := holder.Send(sel("submenu"))
	sidebar := sub.Send(selItemAtIndex, 0)
	// An item AppKit adds itself, as it does Start Dictation to Edit.
	foreign := objc.ID(apple.Class("NSMenuItem")).Send(selAlloc).Send(sel("initWithTitle:action:keyEquivalent:"),
		apple.NSString("Dictation"), objc.SEL(0), apple.NSString(""))
	sub.Send(sel("addItem:"), foreign)
	foreign.Send(selRelease)

	next := append(view(false, 3, driver.MenuEntry{Label: "Link", ID: 5}),
		driver.MenuEntry{Label: "Help", Items: []driver.MenuEntry{{Label: "Docs", ID: 6}}})
	m := s.submenu("", next)
	syncMenu(bar, m)
	m.Send(selRelease)

	if got := bar.Send(selItemAtIndex, 0); got != holder || got.Send(sel("submenu")) != sub || sub.Send(selItemAtIndex, 0) != sidebar {
		t.Fatal("the menu set again replaced items it could keep")
	}
	if n := objc.Send[int](bar, sel("numberOfItems")); n != 2 {
		t.Fatalf("the bar has %d menus, want 2", n)
	}
	if state, tag := objc.Send[int](sidebar, sel("state")), objc.Send[int](sidebar, sel("tag")); state != 0 || tag != 3 {
		t.Errorf("the checked item has state %d and tag %d, want 0 and 3", state, tag)
	}
	var titles []string
	for i := range objc.Send[int](sub, sel("numberOfItems")) {
		titles = append(titles, apple.GoString(sub.Send(selItemAtIndex, i).Send(sel("title"))))
	}
	if want := []string{"Sidebar", "", "Button", "Link", "Dictation"}; !slices.Equal(titles, want) {
		t.Errorf("the View menu has %q, want %q", titles, want)
	}
	help := bar.Send(selItemAtIndex, 1).Send(sel("submenu"))
	if help == 0 || apple.GoString(help.Send(selItemAtIndex, 0).Send(sel("title"))) != "Docs" {
		t.Error("the added menu is missing its item")
	}
}

// TestDockMenu checks the menu applicationDockMenu: hands AppKit: the items, check marks, disabled items, submenus and separators as set, its actions reporting to the surface that set it, without shortcuts, and none once that surface sets nil.
func TestDockMenu(t *testing.T) {
	onMain(func() { dockMenuBuilt(t) })
}

func dockMenuBuilt(t *testing.T) {
	loadAppKit()
	s := &surface{}
	defer func() { clearDockMenu(); delete(menuTargets, s.menuTarget) }()
	s.setDockMenu([]driver.MenuEntry{
		{Separator: true},
		{Label: "New Window", ID: 7, Shortcut: "mod+n"},
		{Label: "Library", Checkable: true, Checked: true, ID: 8},
		{Separator: true},
		{Separator: true},
		{Label: "Paused", Disabled: true, ID: 9},
		{Label: "More", Items: []driver.MenuEntry{{Label: "Open", ID: 10}}},
		{Label: "Locked", Disabled: true, Items: []driver.MenuEntry{{Label: "Inside", ID: 11}}},
		{Separator: true},
	})
	m := dockMenu.menu
	if m == 0 || dockMenu.target != s {
		t.Fatal("no Dock menu was set")
	}
	var titles []string
	for i := range objc.Send[int](m, sel("numberOfItems")) {
		titles = append(titles, apple.GoString(m.Send(selItemAtIndex, i).Send(sel("title"))))
	}
	if want := []string{"New Window", "Library", "", "Paused", "More", "Locked"}; !slices.Equal(titles, want) {
		t.Fatalf("the Dock menu has %q, want %q", titles, want)
	}
	item := func(i int) objc.ID { return m.Send(selItemAtIndex, i) }
	if it := item(0); objc.Send[objc.SEL](it, sel("action")) != selDockMenuChosen || it.Send(sel("target")) != s.menuTarget ||
		objc.Send[int](it, sel("tag")) != 7 || apple.GoString(it.Send(sel("keyEquivalent"))) != "" {
		t.Error("New Window does not report its ID to the surface, or has a shortcut")
	}
	if objc.Send[int](item(1), sel("state")) != 1 {
		t.Error("Library is not checked")
	}
	if objc.Send[objc.SEL](item(3), sel("action")) != selMenuDisabled || objc.Send[objc.SEL](item(5), sel("action")) != selMenuDisabled {
		t.Error("a disabled item or submenu has an action")
	}
	open := item(4).Send(sel("submenu")).Send(selItemAtIndex, 0)
	if objc.Send[objc.SEL](open, sel("action")) != selDockMenuChosen || objc.Send[int](open, sel("tag")) != 10 {
		t.Error("the submenu's item does not report its ID")
	}
	s.setDockMenu(nil)
	if dockMenu.menu != 0 || dockMenu.target != nil {
		t.Error("nil left a Dock menu")
	}
}

// TestMenuRolesRegisterTheShownMenus checks that the Services, Window and Help holders give AppKit the submenus of the bar it shows, at any depth and with no items, still after the menu is set again and synced in place, that a holder losing its role is a new menu, that a bar set after the default menu takes over the one Services menu AppKit keeps, and that the default menu takes back the Help menu.
func TestMenuRolesRegisterTheShownMenus(t *testing.T) {
	onMain(func() { menuRolesRegister(t) })
}

func menuRolesRegister(t *testing.T) {
	loadAppKit()
	nsApp := objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication"))
	s := &surface{}
	defer func() { s.setMenu(nil); delete(menuTargets, s.menuTarget) }()
	menus := func(window bool) []driver.MenuEntry {
		w := driver.MenuEntry{Label: "Window", Items: []driver.MenuEntry{{Label: "Tile", ID: 3}}}
		if window {
			w.Role = driver.MenuWindowMenu
		}
		return []driver.MenuEntry{
			{Label: "App", Items: []driver.MenuEntry{{Label: "Services", Role: driver.MenuServices}, {Separator: true}, {Label: "Quit", Role: driver.MenuQuit}}},
			w,
			{Label: "Help", Role: driver.MenuHelpMenu},
		}
	}
	sub := func(path ...int) objc.ID {
		m := nsApp.Send(sel("mainMenu"))
		for _, i := range path {
			m = m.Send(selItemAtIndex, i).Send(sel("submenu"))
		}
		return m
	}
	check := func(when string) {
		t.Helper()
		for _, c := range []struct {
			get  string
			path []int
		}{{"servicesMenu", []int{0, 0}}, {"windowsMenu", []int{1}}, {"helpMenu", []int{2}}} {
			if got, want := nsApp.Send(sel(c.get)), sub(c.path...); got == 0 || got != want {
				t.Errorf("%s: %s is %v, want the shown bar's %v", when, c.get, got, want)
			}
		}
	}
	s.setMenu(menus(true))
	if s.menu == 0 || nsApp.Send(sel("mainMenu")) != s.menu {
		t.Fatal("the menu is not the main menu")
	}
	check("set")
	services, window := sub(0, 0), sub(1)
	// An item AppKit adds itself, as it does a service.
	foreign := objc.ID(apple.Class("NSMenuItem")).Send(selAlloc).Send(sel("initWithTitle:action:keyEquivalent:"),
		apple.NSString("Service"), objc.SEL(0), apple.NSString(""))
	services.Send(sel("addItem:"), foreign)
	foreign.Send(selRelease)
	s.setMenu(menus(true))
	check("set again")
	if sub(1) != window || sub(0, 0) != services {
		t.Error("the Window and Services menus set again were not synced in place")
	}
	s.setMenu(menus(false))
	if sub(1) == window {
		t.Error("a holder that lost its role kept the Window menu AppKit filled")
	}
	s.setMenu(nil)
	if m := nsApp.Send(sel("helpMenu")); m != 0 {
		t.Errorf("the default menu left the Help menu %v", m)
	}
	s.setMenu(menus(true))
	check("set after the default menu")
	if sub(0, 0) != services || objc.Send[int](services, sel("numberOfItems")) != 1 ||
		apple.GoString(services.Send(selItemAtIndex, 0).Send(sel("title"))) != "Service" {
		t.Error("the new bar did not take over the Services menu AppKit fills")
	}
}

// TestMenuRoleItems checks the actions the new item roles send down the responder chain, Paste and Match Style's key, and a radio's check mark.
func TestMenuRoleItems(t *testing.T) {
	onMain(func() {
		loadAppKit()
		s := &surface{}
		for role, action := range map[driver.MenuRole]string{
			driver.MenuPasteAndMatchStyle: "pasteAsPlainText:", driver.MenuDelete: "delete:", driver.MenuStartSpeaking: "startSpeaking:",
			driver.MenuStopSpeaking: "stopSpeaking:", driver.MenuBringAllToFront: "arrangeInFront:",
		} {
			it := s.item(driver.MenuEntry{Role: role, Label: role.Label()})
			if got := objc.Send[objc.SEL](it, sel("action")); got != sel(action) {
				t.Errorf("%s sends %v, want %s", role.Label(), got, action)
			}
			if role == driver.MenuPasteAndMatchStyle {
				const shift, option, command = 1 << 17, 1 << 19, 1 << 20
				if key, mods := apple.GoString(it.Send(sel("keyEquivalent"))), objc.Send[uint](it, sel("keyEquivalentModifierMask")); key != "v" || mods != shift|option|command {
					t.Errorf("Paste and Match Style's key is %q with %#x, want ⌥⇧⌘V", key, mods)
				}
			}
			it.Send(selRelease)
		}
		it := s.item(driver.MenuEntry{Label: "Movies", Checkable: true, Checked: true, Radio: true, ID: 1})
		if objc.Send[int](it, sel("state")) != 1 {
			t.Error("a checked radio shows no check mark")
		}
		it.Send(selRelease)
		it = s.item(driver.MenuEntry{Label: "Select All", Role: driver.MenuSelectAll, ID: 7})
		if got := objc.Send[objc.SEL](it, sel("action")); got != selMenuRoleChosen || objc.Send[int](it, sel("tag")) != 7 || s.roleFallback[7] != driver.MenuSelectAll {
			t.Errorf("role fallback action = %v, tag %d, role %v", got, objc.Send[int](it, sel("tag")), s.roleFallback[7])
		}
		it.Send(selRelease)
		delete(menuTargets, s.menuTarget)
	})
}
