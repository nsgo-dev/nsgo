//go:build windows

package windows

import (
	"fmt"
	"slices"
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

// TestMenuLeavesOutMacRolesAndDrawsRadios builds a menu with the roles only macOS has, which leave no separator at either end of their group nor an empty submenu, the Speech items, which show only where Windows speaks, a Help holder with no items, which is left out, and radio items, which take MFT_RADIOCHECK.
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
	entries := []driver.MenuEntry{
		{Label: "App", Items: []driver.MenuEntry{{Role: driver.MenuAbout}, {Separator: true}, {Role: driver.MenuServices, Items: []driver.MenuEntry{{Label: "Mine", ID: 1}}},
			{Separator: true}, {Role: driver.MenuQuit, Label: "Quit"}}},
		{Label: "Edit", Items: []driver.MenuEntry{{Role: driver.MenuDelete, Label: "Delete"}, {Separator: true},
			{Label: "Speech", Items: []driver.MenuEntry{{Role: driver.MenuStartSpeaking}, {Role: driver.MenuStopSpeaking}}}}},
		{Label: "Library", Items: []driver.MenuEntry{{Label: "Movies", ID: 2, Radio: true, Checkable: true, Checked: true}, {Label: "Courses", ID: 3, Radio: true, Checkable: true}}},
		{Label: "Window", Role: driver.MenuWindowMenu, Items: []driver.MenuEntry{{Role: driver.MenuMinimize, Label: "Minimize"}, {Separator: true}, {Role: driver.MenuBringAllToFront}}},
		{Label: "Help", Role: driver.MenuHelpMenu},
	}
	for _, c := range []struct {
		speech bool
		want   string
	}{
		{false, "App [Quit\tAlt+F4] Edit [Delete] Library [Movies(o)* Courses(o)] Window [Minimize]"},
		{true, "App [Quit\tAlt+F4] Edit [Delete - Speech [Start Speaking Stop Speaking]] Library [Movies(o)* Courses(o)] Window [Minimize]"},
	} {
		speechAs(t, c.speech)
		menu, _, _ := createPopupMenu.Call()
		var refs []menuRef
		(&window{}).appendMenuTo(menu, entries, &refs)
		if got := strings.Join(items(menu), " "); got != c.want {
			t.Errorf("with speech %v, the menu is %q, want %q", c.speech, got, c.want)
		}
		destroyMenu.Call(menu)
	}
}

// TestMenuYieldFollowsNativeFocus presses Control+R with a MenuNotEditing item and a binding that yields to it: the item takes the key outside any field, the binding in a field and in a page, whose focus WebView2's focus events report, and the item again once the focus leaves.
func TestMenuYieldFollowsNativeFocus(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if _, err := findWebView2(); err != nil {
		t.Skip(err)
	}
	const title = "nsgo-menu-yield"
	app := func(cx *core.Scope) core.Node {
		menu, binding, loaded := core.NewSignal(cx, 0), core.NewSignal(cx, 0), core.NewSignal(cx, false)
		cx.SetApplicationMenu([]core.MenuItem{{Label: "Actions", Items: []core.MenuItem{
			{Label: "Refresh", Shortcut: "mod+r", When: core.MenuNotEditing, OnSelect: func() { menu.Update(func(n int) int { return n + 1 }) }},
		}}})
		cx.OnKeyWith("mod+r", core.KeyOptions{YieldsToMenu: true}, func() { binding.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d|%d|%v|", title, menu.Get(), binding.Get(), loaded.Get())) })
		return core.View{Style: core.Style{Padding: driver.All(10), Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.Input{Style: core.Style{Width: driver.Pt(240), Height: driver.Pt(30)}},
			core.WebView{Source: core.WebHTML("<!doctype html><input>", ""), OnLoad: func(core.WebNavigation) { loaded.Set(true) },
				Style: core.Style{Width: driver.Pt(240), Height: driver.Pt(100)}},
		}}
	}
	hwnd := startApp(t, title, app, 300, 220)
	// The first start of the browser process can take a while.
	waitForTitle(t, hwnd, title+"|0|0|true|", 30*time.Second)
	getState, setState := user32.NewProc("GetKeyboardState"), user32.NewProc("SetKeyboardState")
	press := func(want string) {
		t.Helper()
		onMain(t, hwnd, func(w *window) {
			var saved [256]byte
			getState.Call(uintptr(unsafe.Pointer(&saved)))
			held := saved
			held[vkControl] |= 0x80
			setState.Call(uintptr(unsafe.Pointer(&held)))
			defer setState.Call(uintptr(unsafe.Pointer(&saved)))
			// What WebView2's accelerator event does with Control+R while the page has the focus, else the message loop.
			took := false
			for wv := range w.webs {
				if wv.focused {
					took = wv.acceleratorKey(0, 'R', 1)
					break
				}
			}
			if !took {
				took = w.shortcut('R', 1)
			}
			if !took {
				t.Error("nothing took Control+R")
			}
		})
		waitForTitle(t, hwnd, title+"|"+want+"|true|", 5*time.Second)
	}
	// waitFor waits until the keymap holds the binding as bound says and cond holds, on the main thread.
	waitFor := func(what string, bound bool, cond func(w *window) bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			ok := false
			onMain(t, hwnd, func(w *window) {
				ok = cond(w) && slices.ContainsFunc(w.keys, func(k driver.KeyBinding) bool { return k.Key == "mod+r" }) == bound
			})
			if ok {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal(what)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	page := func(w *window) *webView {
		for wv := range w.webs {
			return wv
		}
		return nil
	}
	press("1|0")
	onMain(t, hwnd, func(w *window) {
		for _, v := range w.views {
			if v != nil && v.field != nil {
				w.setFocus(v.field)
			}
		}
	})
	waitFor("the keymap did not take the binding back in the field", true, func(w *window) bool { return w.focusedField() != nil })
	press("1|1")
	onMain(t, hwnd, func(w *window) { w.blurAll() })
	waitFor("the keymap kept the binding outside the field", false, func(w *window) bool { return w.focusedField() == nil })
	press("2|1")
	onMain(t, hwnd, func(w *window) { page(w).takeFocus() })
	waitFor("the keymap did not take the binding back in the page", true, func(w *window) bool { return page(w).focused })
	press("2|2")
	onMain(t, hwnd, func(w *window) { page(w).focusWindow() })
	waitFor("the keymap kept the binding outside the page", false, func(w *window) bool { return !page(w).focused })
	press("3|2")
}

// TestFocusedPageKeysReachShortcutsFirst types into a focused WebView2 page through the system's input queue, as the user's keyboard does. Control+J, which a KeyAlways binding takes, and Control+K, which a MenuAlways item takes, run the app's handlers and never reach the page; Control+A, whose KeyAuto binding yields to text, Control+L, whose MenuNotEditing item yields too, Control+B, which nothing binds, and a plain letter all reach the page.
func TestFocusedPageKeysReachShortcutsFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if _, err := findWebView2(); err != nil {
		t.Skip(err)
	}
	const title = "nsgo-page-keys"
	app := func(cx *core.Scope) core.Node {
		bound, chosen, yielded, notEditing := core.NewSignal(cx, 0), core.NewSignal(cx, 0), core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		page := core.NewSignal(cx, "")
		cx.SetApplicationMenu([]core.MenuItem{{Label: "Actions", Items: []core.MenuItem{
			{Label: "Palette", Shortcut: "mod+k", When: core.MenuAlways, OnSelect: func() { chosen.Update(func(n int) int { return n + 1 }) }},
			{Label: "Locate", Shortcut: "mod+l", When: core.MenuNotEditing, OnSelect: func() { notEditing.Update(func(n int) int { return n + 1 }) }},
		}}})
		cx.OnKeyWith("mod+j", core.KeyOptions{When: core.KeyAlways}, func() { bound.Update(func(n int) int { return n + 1 }) })
		cx.OnKey("mod+a", func() { yielded.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%d|%d|%d|%d|%s", title, bound.Get(), chosen.Get(), yielded.Get(), notEditing.Get(), page.Get()))
		})
		// The page reports each key that comes down with Control held, the field's value as it changes, and "ready" once it has the focus and its field has it within.
		const html = `<!doctype html><input id=f><script>
const f = document.getElementById('f');
document.addEventListener('keydown', e => { if (e.ctrlKey && e.key !== 'Control') nsgo.postMessage('^' + e.key.toLowerCase()); }, true);
f.addEventListener('input', () => nsgo.postMessage('=' + f.value));
nsgo.postMessage('loaded');
const ready = setInterval(() => {
  f.focus();
  if (document.hasFocus() && document.activeElement === f) { clearInterval(ready); nsgo.postMessage('ready'); }
}, 50);
</script>`
		return core.View{Style: core.Style{Padding: driver.All(10), Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.WebView{Source: core.WebHTML(html, ""), OnMessage: func(data string) { page.Update(func(s string) string { return s + data + " " }) },
				Style: core.Style{Width: driver.Pt(260), Height: driver.Pt(120)}},
		}}
	}
	hwnd := startApp(t, title, app, 300, 200)
	// The first start of the browser process can take a while.
	waitForTitle(t, hwnd, title+"|0|0|0|0|", 30*time.Second)
	if !takeForeground(hwnd) {
		t.Skip("cannot bring the window to the front for the system's input")
	}
	got := waitForTitle(t, hwnd, title+"|0|0|0|0|", 10*time.Second)
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(got, "ready") {
		if time.Now().After(deadline) {
			fg, _, _ := getForegroundWindow.Call()
			t.Fatalf("the page's field did not take the focus (in front: %v): %q", fg == hwnd, got)
		}
		onMain(t, hwnd, func(w *window) {
			for wv := range w.webs {
				if !wv.focused {
					wv.takeFocus()
				}
			}
		})
		time.Sleep(100 * time.Millisecond)
		got = windowText(hwnd)
	}
	onMain(t, hwnd, func(w *window) {
		focused := false
		for wv := range w.webs {
			focused = focused || wv.focused
		}
		if !focused {
			t.Fatal("the window does not know the page has the focus")
		}
	})
	sendKeys(t, hwnd, vkControl, 'J')
	waitForTitle(t, hwnd, title+"|1|0|0|0|", 5*time.Second)
	sendKeys(t, hwnd, vkControl, 'K')
	waitForTitle(t, hwnd, title+"|1|1|0|0|", 5*time.Second)
	sendKeys(t, hwnd, vkControl, 'A')
	sendKeys(t, hwnd, vkControl, 'L')
	sendKeys(t, hwnd, vkControl, 'B')
	sendKeys(t, hwnd, 'X')
	// The page posts in the order the keys came, so its field's value comes after every key before it.
	got = waitForTitle(t, hwnd, title+"|1|1|0|0|", 5*time.Second)
	deadline = time.Now().Add(5 * time.Second)
	for !strings.Contains(got, "=x") {
		if time.Now().After(deadline) {
			t.Fatalf("the page did not type x: %q", got)
		}
		time.Sleep(20 * time.Millisecond)
		got = windowText(hwnd)
	}
	if !strings.HasPrefix(got, title+"|1|1|0|0|") {
		t.Errorf("after the keys the counts are %q, want the binding and the Palette item once and the yielding binding and the Locate item never", got)
	}
	log := got[strings.LastIndex(got, "|")+1:]
	for _, key := range []string{"^j", "^k"} {
		if strings.Contains(log, key) {
			t.Errorf("the page also got %s: %q", key, log)
		}
	}
	for _, key := range []string{"^a", "^l", "^b"} {
		if !strings.Contains(log, key) {
			t.Errorf("the page did not get %s: %q", key, log)
		}
	}
}

var (
	keybdEvent        = user32.NewProc("keybd_event")
	attachThreadInput = user32.NewProc("AttachThreadInput")
	windowThread      = user32.NewProc("GetWindowThreadProcessId")
	currentThread     = kernel32.NewProc("GetCurrentThreadId")
)

// takeForeground makes hwnd the foreground window, joining the input of the window in front for the moment Windows' foreground lock asks it to, and reports whether it is.
func takeForeground(hwnd uintptr) bool {
	for range 20 {
		if fg, _, _ := getForegroundWindow.Call(); fg == hwnd {
			return true
		}
		fg, _, _ := getForegroundWindow.Call()
		own, _, _ := currentThread.Call()
		other, _, _ := windowThread.Call(fg, 0)
		if other != 0 && other != own {
			attachThreadInput.Call(own, other, 1)
		}
		setForegroundWindow.Call(hwnd)
		if other != 0 && other != own {
			attachThreadInput.Call(own, other, 0)
		}
		time.Sleep(100 * time.Millisecond)
	}
	fg, _, _ := getForegroundWindow.Call()
	return fg == hwnd
}

// sendKeys presses keys in order and lets them go in reverse through the system's input queue, after checking hwnd is still in front so the keys go nowhere else.
func sendKeys(t *testing.T, hwnd uintptr, keys ...byte) {
	t.Helper()
	if fg, _, _ := getForegroundWindow.Call(); fg != hwnd {
		t.Fatal("the window left the front before the keys were sent")
	}
	const keyUp = 0x2
	for _, k := range keys {
		scan, _, _ := mapVirtualKeyW.Call(uintptr(k), 0)
		keybdEvent.Call(uintptr(k), scan, 0, 0)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		scan, _, _ := mapVirtualKeyW.Call(uintptr(keys[i]), 0)
		keybdEvent.Call(uintptr(keys[i]), scan, keyUp, 0)
	}
	time.Sleep(150 * time.Millisecond)
}
