package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestKeyBindingsFireAsTheirOptionsSay(t *testing.T) {
	keymap := []driver.KeyBinding{
		{Key: "down", InText: !EditsText("down"), ID: 1},
		{Key: "mod+k", InText: !EditsText("mod+k"), ID: 2},
		{Key: "mod+z", InText: true, Repeat: true, ID: 3},
	}
	for _, c := range []struct {
		names           []string
		editing, repeat bool
		id              uint32
		consumed        bool
	}{
		{[]string{"down"}, false, false, 1, true},
		{[]string{"down"}, true, false, 0, false}, // the field moves its caret
		{[]string{"mod+k", "meta+k"}, true, false, 2, true},
		{[]string{"mod+k", "meta+k"}, false, true, 0, true}, // held: consumed, fires once
		{[]string{"mod+z"}, true, true, 3, true},
		{[]string{"x"}, false, false, 0, false},
	} {
		var focus driver.KeyUse
		if c.editing {
			focus = driver.KeyUseText
		}
		id, consumed := driver.FindKey(keymap, c.names, focus, c.repeat)
		if id != c.id || consumed != c.consumed {
			t.Errorf("FindKey(%v, editing %v, repeat %v) = %d, %v; want %d, %v", c.names, c.editing, c.repeat,
				id, consumed, c.id, c.consumed)
		}
	}
}

// TestBindingsYieldToTheFocusedControl holds KeyAuto bindings to leave a focused control the keys it uses, and KeyNotEditing and KeyAlways ones to take them, KeyNotEditing's none from a text input (#205).
func TestBindingsYieldToTheFocusedControl(t *testing.T) {
	var keymap []driver.KeyBinding
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		for _, b := range []struct {
			key  string
			when KeyWhen
		}{{"space", KeyAuto}, {"right", KeyAuto}, {"mod+enter", KeyAuto}, {"enter", KeyNotEditing},
			{"down", KeyNotEditing}, {"home", KeyAlways}, {"shift+space", KeyAuto}} {
			cx.OnKeyWith(b.key, KeyOptions{When: b.when}, func() {})
		}
		return View{}
	})
	for _, it := range h.commit().Items {
		if it.Kind == driver.ItemKeymap {
			keymap = it.Keys
		}
	}
	if len(keymap) != 7 {
		t.Fatalf("keymap = %v", keymap)
	}
	press, adjust, sel := driver.KeyUsePress, driver.KeyUseAdjust, driver.KeyUseSelect
	want := []driver.KeyUse{press, adjust | sel, 0, 0, 0, 0, 0}
	for i, b := range keymap {
		if b.Yields != want[i] {
			t.Errorf("%s yields %b, want %b", b.Key, b.Yields, want[i])
		}
	}
	button := driver.FocusKeyUse(driver.KindView, driver.RoleButton, true)
	slider := driver.FocusKeyUse(driver.KindView, driver.RoleAdjustable, true)
	tab := driver.FocusKeyUse(driver.KindView, driver.RoleTab, true)
	plain := driver.FocusKeyUse(driver.KindView, driver.RoleNone, false)
	for _, c := range []struct {
		name  string
		focus driver.KeyUse
		fires bool
	}{
		{"space", button, false}, {"space", plain, true}, {"space", slider, false},
		{"right", slider, false}, {"right", tab, false}, {"right", button, true},
		{"mod+enter", button, true}, {"enter", button, true}, {"enter", driver.KeyUseText, false},
		{"down", slider, true}, {"down", driver.KeyUseText, false},
		{"home", slider, true}, {"home", driver.KeyUseText, true}, {"shift+space", button, true},
	} {
		id, consumed := driver.FindKey(keymap, []string{c.name}, c.focus, false)
		i := slices.IndexFunc(keymap, func(b driver.KeyBinding) bool { return b.ID == id })
		if fired := i >= 0 && keymap[i].Key == c.name && consumed; fired != c.fires || !fired && consumed {
			t.Errorf("%s with focus %b = %d, %v; want firing %v", c.name, c.focus, id, consumed, c.fires)
		}
	}
	if got := driver.FocusKeyUse(driver.KindInput, driver.RoleNone, false); got != driver.KeyUseText {
		t.Errorf("a text input uses %b", got)
	}
}

// TestAKeyRunsTheBindingThePlatformChose holds the engine to run, of a key's bindings, the one the platform chose for the focus it matched by: the control's own, not a newer one that yields to it (#205).
func TestAKeyRunsTheBindingThePlatformChose(t *testing.T) {
	var log []string
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		cx.OnKeyWith("right", KeyOptions{When: KeyNotEditing}, func() { log = append(log, "slider") })
		cx.OnKey("right", func() { log = append(log, "app") })
		return View{}
	})
	h.commit()
	for _, c := range []struct {
		focus driver.KeyUse
		want  string
	}{{driver.KeyUseAdjust | driver.KeyUsePress, "slider"}, {driver.KeyUsePress, "app"}, {0, "app"}} {
		log = nil
		h.pressKey("right", c.focus)
		if !slices.Equal(log, []string{c.want}) {
			t.Errorf("right over focus %b ran %q, want %q", c.focus, log, c.want)
		}
	}
}

// TestAKeyRunsTheBindingThePlatformMatched holds the engine to run the binding the platform matched in a focused text input, a KeyAlways one, not a newer KeyAuto or KeyNotEditing binding of the key that does not fire there, and the newest one with no field focused (#204).
func TestAKeyRunsTheBindingThePlatformMatched(t *testing.T) {
	for _, key := range []string{"down", "escape", "enter"} {
		for _, when := range []KeyWhen{KeyAuto, KeyNotEditing} {
			var ran []string
			h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
				cx.OnKeyWith(key, KeyOptions{When: KeyAlways}, func() { ran = append(ran, "always") })
				cx.OnKeyWith(key, KeyOptions{When: when}, func() { ran = append(ran, "newer") })
				return View{}
			})
			h.commit()
			if !h.pressKey(key, driver.KeyUseText) {
				t.Errorf("%s (%d) in a field: not consumed", key, when)
			}
			h.pressKey(key, 0)
			if want := []string{"always", "newer"}; !slices.Equal(ran, want) {
				t.Errorf("%s (%d), in a field and then not: ran %q, want %q", key, when, ran, want)
			}
		}
	}
}

// TestAStaleKeyPressRunsOnlyItsBinding holds the engine to run, for a press the platform matched by a keymap that has changed since, the binding it matched or none: never one bound since, nor an older one of the key in place of one removed or resting (#204).
func TestAStaleKeyPressRunsOnlyItsBinding(t *testing.T) {
	var ran []string
	var newer, covered *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		newer, covered = NewSignal(cx, true), NewSignal(cx, false)
		cx.OnKey("down", func() { ran = append(ran, "older") })
		return View{Children: []Node{
			Show{When: newer.Get, Then: func(cx *Scope) Node {
				cx.OnKey("down", func() { ran = append(ran, "newer") })
				return View{}
			}},
			Show{When: covered.Get, Then: func(cx *Scope) Node { return Modal{Children: []Node{Text{Content: "dialog"}}} }},
		}}
	})
	h.commit()
	match := func() uint32 {
		id, _ := driver.FindKey(h.tree.keymap(), []string{"down"}, 0, false)
		return id
	}
	// The newer binding is matched, then removed before the press arrives: nothing runs, not the older one.
	id := match()
	newer.Set(false)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Count: id})
	if len(ran) != 0 {
		t.Fatalf("a press matched to a removed binding ran %q", ran)
	}
	// The older binding is matched, then a newer one of the key comes: the matched one runs.
	id = match()
	newer.Set(true)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Count: id})
	if want := []string{"older"}; !slices.Equal(ran, want) {
		t.Fatalf("a press matched before a newer binding came ran %q, want %q", ran, want)
	}
	// The newer binding is matched, then a modal covers it: it rests, and nothing runs.
	ran = nil
	id = match()
	covered.Set(true)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Count: id})
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Count: 0})
	if len(ran) != 0 {
		t.Fatalf("a press matched to a binding a modal has since covered ran %q", ran)
	}
}

// TestKeyIDsAreNotReused holds every binding to a new ID, so a press matched to one gone cannot run one bound in its place.
func TestKeyIDsAreNotReused(t *testing.T) {
	var on *Signal[bool]
	seen := map[uint32]bool{}
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		on = NewSignal(cx, true)
		return Show{When: on.Get, Then: func(cx *Scope) Node {
			cx.OnKey("down", func() {})
			return View{}
		}}
	})
	for range 3 {
		keys := h.tree.keymap()
		h.commit()
		if len(keys) != 1 || keys[0].ID == 0 || seen[keys[0].ID] {
			t.Fatalf("keymap %+v after IDs %v", keys, seen)
		}
		seen[keys[0].ID] = true
		on.Set(false)
		h.commit()
		on.Set(true)
		h.commit()
	}
}

// A held key repeats as the binding the platform matched says, not as the newest of the key does (#204).
func TestARepeatFollowsTheMatchedBinding(t *testing.T) {
	keymap := []driver.KeyBinding{
		{Key: "down", InText: true, Repeat: true, ID: 1},
		{Key: "down", ID: 2},
	}
	if id, consumed := driver.FindKey(keymap, []string{"down"}, driver.KeyUseText, true); id != 1 || !consumed {
		t.Errorf("held in a field = %d, %v; want the repeating binding 1", id, consumed)
	}
	if id, consumed := driver.FindKey(keymap, []string{"down"}, 0, true); id != 0 || !consumed {
		t.Errorf("held with no field = %d, %v; want consumed without firing", id, consumed)
	}
}

func TestKeyLabelsFollowThePlatform(t *testing.T) {
	for _, c := range []struct {
		key   string
		os    OS
		apple bool
		want  string
	}{
		{"mod+shift+l", OSMacOS, true, "⇧⌘L"},
		{"mod+shift+l", OSWindows, false, "Ctrl+Shift+L"},
		{"ctrl+alt+escape", OSMacOS, true, "⌃⌥⎋"},
		{"meta+k", OSWindows, false, "Win+K"},
		{"meta+k", OSLinux, false, "Super+K"},
		{"f5", OSWeb, false, "F5"},
		{"mod+enter", OSWeb, true, "⌘↩"},
	} {
		if got := KeyLabel(c.key, c.os, c.apple); got != c.want {
			t.Errorf("KeyLabel(%q, %s) = %q, want %q", c.key, c.os, got, c.want)
		}
	}
}

func TestBackGoesToTheNewestHandlerThatTakesIt(t *testing.T) {
	var log []string
	var inner *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		inner = NewSignal(cx, true)
		cx.OnBackPress(func() bool { log = append(log, "outer"); return true })
		return Show{When: inner.Get, Then: func(cx *Scope) Node {
			cx.OnBackPress(func() bool { log = append(log, "inner passes"); return false })
			return View{}
		}}
	})
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	inner.Set(false)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if want := []string{"inner passes", "outer", "outer"}; !slices.Equal(log, want) {
		t.Fatalf("log = %q, want %q", log, want)
	}
}

// pressKey is what a platform does with a press of the key name while the focused view uses focus: it matches the press by the keymap the core sends (driver.FindKey) and reports the binding that fires, if any, by its ID. It returns whether the press is consumed.
func (h *host) pressKey(name string, focus driver.KeyUse) bool {
	id, consumed := driver.FindKey(h.tree.keymap(), []string{name}, focus, false)
	if id != 0 {
		h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Count: id})
	}
	return consumed
}

// withoutIDs is keys with their IDs cleared, to compare the rest.
func withoutIDs(keys []driver.KeyBinding) []driver.KeyBinding {
	out := slices.Clone(keys)
	for i := range out {
		out[i].ID = 0
	}
	return out
}

// sentKeys returns the keys of the keymap batch b sends, and whether it sends one.
func sentKeys(b *driver.MountBatch) ([]string, bool) {
	var keys []string
	sent := false
	for _, it := range b.Items {
		if it.Kind == driver.ItemKeymap {
			sent, keys = true, nil
			for _, k := range it.Keys {
				keys = append(keys, k.Key)
			}
		}
	}
	return keys, sent
}

// A modal shadows the bindings of what it covers, in the keymap the platform matches by and in the core, so Escape reaches its OnRequestClose; what is in it, what shows only it, an OverModals binding and a Topmost overlay above it keep theirs, and closing it brings the page's back.
func TestKeysFollowVisibleModals(t *testing.T) {
	var open *Signal[bool]
	var log []string
	add := func(s string) func() { return func() { log = append(log, s) } }
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		cx.OnKey("space", add("page space"))
		cx.OnKey("escape", add("page escape"))
		cx.OnKeyWith("f1", KeyOptions{OverModals: true}, add("global f1"))
		held := NewHolder(cx, func(cx *Scope) Node {
			return Func(func(cx *Scope) Node {
				cx.OnKey("h", add("held h"))
				return Text{Content: "held"}
			})
		})
		return View{Children: []Node{
			Text{Content: "page"},
			Overlay{Topmost: true, Children: []Node{Func(func(cx *Scope) Node {
				cx.OnKey("t", add("topmost t"))
				return Text{Content: "dev"}
			})}},
			Show{When: open.Get, Then: func(cx *Scope) Node {
				// The owner that shows only the modal binds as the dialog does.
				cx.OnKey("enter", add("dialog enter"))
				return Modal{OnRequestClose: func(string) { log = append(log, "close"); open.Set(false) }, Children: []Node{
					Func(func(cx *Scope) Node {
						cx.OnKey("space", add("dialog space"))
						return Text{Content: "dialog"}
					}),
					held.Place(),
				}}
			}},
		}}
	})
	press := func(keys ...string) {
		t.Helper()
		for _, k := range keys {
			h.pressKey(k, 0)
		}
	}
	keys, _ := sentKeys(h.commit())
	if want := []string{"f1", "t", "enter", "space", "h"}; !slices.Equal(keys, want) {
		t.Fatalf("keymap under the modal = %q, want %q", keys, want)
	}
	press("space", "escape", "f1", "enter", "t", "h")
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if want := []string{"dialog space", "global f1", "dialog enter", "topmost t", "held h", "close"}; !slices.Equal(log, want) {
		t.Fatalf("with the modal: log = %q, want %q", log, want)
	}
	log = nil
	keys, _ = sentKeys(h.commit())
	if want := []string{"space", "escape", "f1", "t"}; !slices.Equal(keys, want) {
		t.Fatalf("keymap after the modal = %q, want %q", keys, want)
	}
	press("space", "escape", "h")
	if want := []string{"page space", "page escape"}; !slices.Equal(log, want) {
		t.Fatalf("after the modal: log = %q, want %q", log, want)
	}
}

// A binding or back handler in a Holder's content takes part only while a place shows it, whatever owner made the holder.
func TestKeysAndBackFollowHolderPlacement(t *testing.T) {
	var tab *Signal[int]
	var ran []string
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		tab = NewSignal(cx, 0)
		pane := func(name string) *Holder {
			return NewHolder(cx, func(cx *Scope) Node {
				cx.OnKey("down", func() { ran = append(ran, name+" down") })
				cx.OnBackPress(func() bool { ran = append(ran, name+" back"); return true })
				// The content's own views are a Func's: the holder's owner has only its region.
				return Func(func(cx *Scope) Node { return Text{Content: name} })
			})
		}
		a, b := pane("a"), pane("b")
		return Dynamic(func(cx *Scope) Node {
			if tab.Get() == 0 {
				return a.Place()
			}
			return b.Place()
		})
	})
	testPane := func(name string) {
		t.Helper()
		keys, _ := sentKeys(h.commit())
		if !slices.Equal(keys, []string{"down"}) {
			t.Fatalf("pane %s shows, keymap = %q", name, keys)
		}
		ran = nil
		h.pressKey("down", 0)
		h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
		want := []string{name + " down", name + " back"}
		if !slices.Equal(ran, want) {
			t.Fatalf("visible pane %s ran %q, want %q", name, ran, want)
		}
	}
	testPane("a")
	tab.Set(1)
	testPane("b")
	tab.Set(0)
	testPane("a")
}

// What an owner shows is its own views, not those of the owners under it: an app's binding stays while content deeper than all it shows waits out of the tree.
func TestRootKeysSurviveDeeperUnplacedContent(t *testing.T) {
	var tab *Signal[int]
	var root int
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		tab = NewSignal(cx, 1)
		cx.OnKey("x", func() { root++ })
		deep := NewHolder(cx, func(cx *Scope) Node {
			return Func(func(cx *Scope) Node {
				return Func(func(cx *Scope) Node {
					return View{Children: []Node{Func(func(cx *Scope) Node { return Text{Content: "deep"} })}}
				})
			})
		})
		return Dynamic(func(cx *Scope) Node {
			if tab.Get() == 1 {
				return deep.Place()
			}
			return Text{Content: "shallow"}
		})
	})
	h.commit()
	tab.Set(0)
	if keys, _ := sentKeys(h.commit()); !slices.Equal(keys, []string{"x"}) {
		t.Fatalf("keymap = %q, want the app's x", keys)
	}
	h.pressKey("x", 0)
	if root != 1 {
		t.Fatalf("root binding ran %d times with a deeper unplaced holder, want 1", root)
	}
}

// DisplayNone hides a view's bindings, and the keymap follows it as the style changes.
func TestKeysFollowDisplayNone(t *testing.T) {
	var hidden *Signal[bool]
	var ran int
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		hidden = NewSignal(cx, false)
		return View{StyleFunc: func() Style {
			if hidden.Get() {
				return Style{Display: DisplayNone}
			}
			return Style{}
		}, Children: []Node{Func(func(cx *Scope) Node {
			cx.OnKey("down", func() { ran++ })
			return Text{Content: "pane"}
		})}}
	})
	if keys, _ := sentKeys(h.commit()); !slices.Equal(keys, []string{"down"}) {
		t.Fatalf("shown: keymap = %q", keys)
	}
	hidden.Set(true)
	if keys, sent := sentKeys(h.commit()); !sent || len(keys) != 0 {
		t.Fatalf("hidden: keymap = %q (sent %v), want an empty one", keys, sent)
	}
	h.pressKey("down", 0)
	hidden.Set(false)
	if keys, _ := sentKeys(h.commit()); !slices.Equal(keys, []string{"down"}) {
		t.Fatalf("shown again: keymap = %q", keys)
	}
	h.pressKey("down", 0)
	if ran != 1 {
		t.Fatalf("ran %d, want 1: only while shown", ran)
	}
}

// A navigation stack's screen below the top is hidden, so its bindings rest until it is on top again.
func TestKeysFollowTheTopScreen(t *testing.T) {
	var pushed *Signal[bool]
	var log []string
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		pushed = NewSignal(cx, false)
		list := Screen{Children: []Node{Func(func(cx *Scope) Node {
			cx.OnKey("down", func() { log = append(log, "list down") })
			return Text{Content: "list"}
		})}}
		return Stack{Children: []Node{list, Show{When: pushed.Get, Then: func(*Scope) Node { return Screen{Children: []Node{Text{Content: "detail"}}} }}}}
	})
	press := func(want ...string) {
		t.Helper()
		log = nil
		h.commit()
		h.pressKey("down", 0)
		if !slices.Equal(log, want) {
			t.Fatalf("log = %q, want %q", log, want)
		}
	}
	press("list down")
	pushed.Set(true)
	press()
	pushed.Set(false)
	press("list down")
}

// A modal that binds nothing still takes the page's bindings out of the keymap as it mounts, and puts them back as it goes.
func TestKeymapFollowsAModalThatBindsNothing(t *testing.T) {
	var open *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		open = NewSignal(cx, false)
		cx.OnKey("space", func() {})
		return View{Children: []Node{
			Text{Content: "page"},
			Show{When: open.Get, Then: func(*Scope) Node { return Modal{Children: []Node{Text{Content: "dialog"}}} }},
		}}
	})
	if keys, _ := sentKeys(h.commit()); !slices.Equal(keys, []string{"space"}) {
		t.Fatalf("before the modal: keymap = %q", keys)
	}
	open.Set(true)
	if keys, sent := sentKeys(h.commit()); !sent || len(keys) != 0 {
		t.Fatalf("with the modal: keymap = %q (sent %v), want an empty one", keys, sent)
	}
	open.Set(false)
	if keys, sent := sentKeys(h.commit()); !sent || !slices.Equal(keys, []string{"space"}) {
		t.Fatalf("after the modal: keymap = %q (sent %v), want the page's space", keys, sent)
	}
}

// A YieldsToMenu binding leaves its key out of the keymap, which platforms match before the application menu, while an enabled item of the menu has the key as its shortcut, written with "mod" or the platform's own Command or Control, a role's included; a disabled item or one in a disabled submenu leaves it the key, and it takes the key back as the menu goes. A binding without the option keeps its key before the menu. Where no menu shows, on iOS, Android and the web, every binding keeps its key.
func TestBindingsYieldToTheMenuAsAsked(t *testing.T) {
	for _, os := range []OS{OSMacOS, OSWindows, OSLinux, OSIOS, OSAndroid, OSWeb} {
		command := map[OS]string{OSMacOS: "Cmd+R", OSIOS: "Cmd+R"}[os]
		if command == "" {
			command = "Ctrl+R"
		}
		for _, c := range []struct {
			name string
			menu []MenuItem
			want []string
		}{
			{"no menu", nil, []string{"mod+r", "mod+q", "mod+s"}},
			{"mod+r", []MenuItem{{Label: "Run", Shortcut: "mod+r"}}, []string{"mod+q", "mod+s"}},
			{command, []MenuItem{{Label: "Run", Shortcut: command}}, []string{"mod+q", "mod+s"}},
			{"disabled", []MenuItem{{Label: "Run", Shortcut: "mod+r", Disabled: true}}, []string{"mod+r", "mod+q", "mod+s"}},
			{"in a disabled submenu", []MenuItem{{Label: "More", Disabled: true, Items: []MenuItem{{Label: "Run", Shortcut: "mod+r"}}}}, []string{"mod+r", "mod+q", "mod+s"}},
			{"other keys", []MenuItem{{Label: "Rerun", Shortcut: "mod+shift+r"}, {Label: "Save", Shortcut: "mod+s"}}, []string{"mod+r", "mod+q", "mod+s"}},
			// Quit's own shortcut is Command+Q on a Mac, Alt+F4 elsewhere.
			{"a role", []MenuItem{{Role: MenuQuit}}, map[bool][]string{true: {"mod+r", "mod+s"}, false: {"mod+r", "mod+q", "mod+s"}}[os == OSMacOS]},
		} {
			var shown *Signal[bool]
			var ran []string
			h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
				shown = NewSignal(cx, false)
				for _, k := range []string{"mod+r", "mod+q"} {
					cx.OnKeyWith(k, KeyOptions{YieldsToMenu: true}, func() { ran = append(ran, k) })
				}
				cx.OnKey("mod+s", func() { ran = append(ran, "mod+s") })
				return Show{When: shown.Get, Then: func(cx *Scope) Node {
					cx.SetApplicationMenu([]MenuItem{{Label: "File", Items: c.menu}})
					return View{}
				}}
			})
			h.tree.SetEnvironment(Environment{OS: os, Idiom: IdiomDesktop})
			keys, _ := sentKeys(h.commit())
			if c.menu != nil {
				shown.Set(true)
				keys, _ = sentKeys(h.commit())
			}
			if os != OSMacOS && os != OSWindows && os != OSLinux {
				c.want = []string{"mod+r", "mod+q", "mod+s"}
			}
			if !slices.Equal(keys, c.want) {
				t.Errorf("%s, %s: keymap = %q, want %q", os, c.name, keys, c.want)
			}
			h.pressKey("mod+r", 0)
			if fired := slices.Contains(ran, "mod+r"); fired != slices.Contains(c.want, "mod+r") {
				t.Errorf("%s, %s: mod+r fired %v", os, c.name, fired)
			}
			if c.menu == nil {
				continue
			}
			shown.Set(false)
			if keys, sent := sentKeys(h.commit()); !sent || !slices.Equal(keys, []string{"mod+r", "mod+q", "mod+s"}) {
				t.Errorf("%s, %s: after the menu, keymap = %q (sent %v), want every binding", os, c.name, keys, sent)
			}
		}
	}
}
