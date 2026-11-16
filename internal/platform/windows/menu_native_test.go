//go:build windows

package windows

import (
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestHiddenTitleBarMenuPopsUp opens the application menu of a window with a hidden title bar, which has no menu bar: Alt alone and F10 open it as a popup whose items do what the bar's do, Alt with a top-level item's first letter opens its menu or chooses it, and Alt with any other key, or after a click, stays Windows'. With the system title bar the bar takes the keys as before.
func TestHiddenTitleBarMenuPopsUp(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	chosen := make(chan string, 4)
	scopes := make(chan *core.Scope, 1)
	app := func(cx *core.Scope) core.Node {
		cx.SetApplicationMenu([]core.MenuItem{
			{Label: "File", Items: []core.MenuItem{{Label: "New", OnSelect: func() { chosen <- "New" }}}},
			{Label: "Sync", OnSelect: func() { chosen <- "Sync" }},
		})
		scopes <- cx
		return core.View{Style: core.Style{Background: driver.RGB(0xffffff)}}
	}
	hwnd := startApp(t, "nsgo-hidden-title-menu", app, 360, 240)
	var cx *core.Scope
	select {
	case cx = <-scopes:
	case <-time.After(5 * time.Second):
		t.Fatal("the app did not start")
	}
	menuBar := func() uintptr {
		var m uintptr
		onMain(t, hwnd, func(*window) { m, _, _ = getMenu.Call(hwnd) })
		return m
	}
	// waitFor waits until cond holds on the main thread.
	waitFor := func(what string, cond func(w *window) bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			ok := false
			onMain(t, hwnd, func(w *window) { ok = cond(w) })
			if ok {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal(what)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	// The system title bar shows the bar, which takes Alt into its menu mode.
	waitFor("no menu bar", func(w *window) bool { return w.chrome.bar != 0 })
	onMain(t, hwnd, func(w *window) {
		w.menuBarKey(wmSysKeyDown, vkMenu, 1<<29)
		if w.menuBarKey(wmSysKeyUp, vkMenu, 1<<29|1<<30|1<<31) || w.menuChar('f') || len(w.popups) > 0 {
			t.Error("Alt was taken from a shown menu bar")
		}
	})

	cx.Post(func() { cx.SetTitleBar(core.TitleBarHidden, 40) })
	waitFor("the title bar did not hide", func(w *window) bool { return w.chrome.titleBar == driver.TitleBarHidden })
	if menuBar() != 0 {
		t.Error("a window with a hidden title bar shows a menu bar")
	}
	// popup sends keys and waits for the menu they opened, returning its entries.
	popup := func(keys string, send func()) []driver.MenuEntry {
		t.Helper()
		send()
		var shown []driver.MenuEntry
		waitFor(keys+" opened no menu", func(w *window) bool {
			if w.popupTracking {
				shown = w.popupShown
			}
			return shown != nil
		})
		return shown
	}
	// end ends the menu's tracking, choosing item id unless it is zero.
	end := func(id uint32) {
		t.Helper()
		onMain(t, hwnd, func(w *window) {
			if id != 0 {
				w.popupChoice = &id
			}
			endMenu.Call()
		})
		waitFor("the menu did not close", func(w *window) bool { return !w.popupTracking && len(w.popups) == 0 })
	}
	wantChosen := func(keys, want string) {
		t.Helper()
		select {
		case got := <-chosen:
			if got != want {
				t.Errorf("%s chose %s, want %s", keys, got, want)
			}
		case <-time.After(3 * time.Second):
			t.Errorf("%s did not choose %s", keys, want)
		}
	}

	shown := popup("Alt", func() {
		postMessageW.Call(hwnd, wmSysKeyDown, vkMenu, 1<<29)
		postMessageW.Call(hwnd, wmSysKeyUp, vkMenu, 1<<29|1<<30|1<<31)
	})
	if len(shown) != 2 || shown[0].Label != "File" || len(shown[0].Items) != 1 || shown[1].Label != "Sync" {
		t.Fatalf("Alt opened %+v, want the application menu", shown)
	}
	end(shown[0].Items[0].ID)
	wantChosen("Alt", "New")

	if shown := popup("F10", func() { postMessageW.Call(hwnd, wmSysKeyDown, vkF10, 0) }); len(shown) != 2 {
		t.Errorf("F10 opened %+v, want the application menu", shown)
	}
	end(0)

	shown = popup("Alt+F", func() { postMessageW.Call(hwnd, wmSysChar, 'F', 1<<29) })
	if len(shown) != 1 || shown[0].Label != "New" {
		t.Fatalf("Alt+F opened %+v, want the File menu", shown)
	}
	end(shown[0].ID)
	wantChosen("Alt+F", "New")

	postMessageW.Call(hwnd, wmSysChar, 's', 1<<29)
	wantChosen("Alt+S", "Sync")

	onMain(t, hwnd, func(w *window) {
		// Alt+F4's key is no mnemonic's, and DefWindowProc closes the window with it.
		if w.menuBarKey(wmSysKeyDown, 0x73, 1<<29) || w.menuChar('x') || len(w.popups) > 0 || w.popupTracking {
			t.Error("Alt with a key no menu has opened a menu")
		}
		// Alt+click: a press in the window makes Alt no longer alone.
		w.menuBarKey(wmSysKeyDown, vkMenu, 1<<29)
		w.hideMenuBar()
		if w.menuBarKey(wmSysKeyUp, vkMenu, 1<<29|1<<30|1<<31) || len(w.popups) > 0 {
			t.Error("releasing Alt after a click opened the menu")
		}
	})
}

// TestMenuLeavesOutMacRolesAndDrawsRadios builds a menu with the roles only macOS has, which leave no separator at either end of their group nor an empty submenu, a Help holder with no items, which is left out, and radio items, which take MFT_RADIOCHECK.
func TestMenuLeavesOutMacRolesAndDrawsRadios(t *testing.T) {
	getItem := user32.NewProc("GetMenuItemInfoW")
	getString := user32.NewProc("GetMenuStringW")
	count := user32.NewProc("GetMenuItemCount")
	type menuItemInfo struct {
		cbSize, fMask, fType, fState, wID    uint32
		hSubMenu, hbmpChecked, hbmpUnchecked uintptr
		dwItemData, dwTypeData               uintptr
		cch                                  uint32
		hbmpItem                             uintptr
	}
	// items returns menu's items as labels, "-" for a separator, with "(o)" for a radio and "*" when checked, and their submenus.
	var items func(menu uintptr) []string
	items = func(menu uintptr) []string {
		var out []string
		n, _, _ := count.Call(menu)
		for i := range n {
			mi := menuItemInfo{fMask: 0x100 | 0x1 | 0x4} // MIIM_FTYPE, MIIM_STATE, MIIM_SUBMENU
			mi.cbSize = uint32(unsafe.Sizeof(mi))
			getItem.Call(menu, i, 1, uintptr(unsafe.Pointer(&mi)))
			if mi.fType&mfSeparator != 0 {
				out = append(out, "-")
				continue
			}
			buf := make([]uint16, 128)
			getString.Call(menu, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0x400)
			s := syscall.UTF16ToString(buf)
			if mi.fType&mfRadioCheck != 0 {
				s += "(o)"
			}
			if mi.fState&mfChecked != 0 {
				s += "*"
			}
			out = append(out, s)
			if mi.hSubMenu != 0 {
				out = append(out, "["+strings.Join(items(mi.hSubMenu), " ")+"]")
			}
		}
		return out
	}
	menu, _, _ := createPopupMenu.Call()
	defer destroyMenu.Call(menu)
	w := &window{}
	var refs []menuRef
	w.appendMenuTo(menu, []driver.MenuEntry{
		{Label: "App", Items: []driver.MenuEntry{{Role: driver.MenuAbout}, {Separator: true}, {Role: driver.MenuServices, Items: []driver.MenuEntry{{Label: "Mine", ID: 1}}},
			{Separator: true}, {Role: driver.MenuQuit, Label: "Quit"}}},
		{Label: "Edit", Items: []driver.MenuEntry{{Role: driver.MenuDelete, Label: "Delete"}, {Separator: true},
			{Label: "Speech", Items: []driver.MenuEntry{{Role: driver.MenuStartSpeaking}, {Role: driver.MenuStopSpeaking}}}}},
		{Label: "Library", Items: []driver.MenuEntry{{Label: "Movies", ID: 2, Radio: true, Checkable: true, Checked: true}, {Label: "Courses", ID: 3, Radio: true, Checkable: true}}},
		{Label: "Window", Role: driver.MenuWindowMenu, Items: []driver.MenuEntry{{Role: driver.MenuMinimize, Label: "Minimize"}, {Separator: true}, {Role: driver.MenuBringAllToFront}}},
		{Label: "Help", Role: driver.MenuHelpMenu},
	}, &refs)
	got := strings.Join(items(menu), " ")
	if want := "App [Quit\tAlt+F4] Edit [Delete] Library [Movies(o)* Courses(o)] Window [Minimize]"; got != want {
		t.Errorf("the menu is %q, want %q", got, want)
	}
}
