package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestKeyBindingsFireAsTheirOptionsSay(t *testing.T) {
	keymap := []driver.KeyBinding{
		{Key: "down", InText: !EditsText("down")},
		{Key: "mod+k", InText: !EditsText("mod+k")},
		{Key: "mod+z", InText: true, Repeat: true},
	}
	for _, c := range []struct {
		names           []string
		editing, repeat bool
		key             string
		consumed        bool
	}{
		{[]string{"down"}, false, false, "down", true},
		{[]string{"down"}, true, false, "", false}, // the field moves its caret
		{[]string{"mod+k", "meta+k"}, true, false, "mod+k", true},
		{[]string{"mod+k", "meta+k"}, false, true, "", true}, // held: consumed, fires once
		{[]string{"mod+z"}, true, true, "mod+z", true},
		{[]string{"x"}, false, false, "", false},
	} {
		var focus driver.KeyUse
		if c.editing {
			focus = driver.KeyUseText
		}
		key, consumed := driver.FindKey(keymap, c.names, focus, c.repeat)
		if key != c.key || consumed != c.consumed {
			t.Errorf("FindKey(%v, editing %v, repeat %v) = %q, %v; want %q, %v", c.names, c.editing, c.repeat,
				key, consumed, c.key, c.consumed)
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
		key, consumed := driver.FindKey(keymap, []string{c.name}, c.focus, false)
		if fired := key == c.name && consumed; fired != c.fires || !fired && consumed {
			t.Errorf("%s with focus %b = %q, %v; want firing %v", c.name, c.focus, key, consumed, c.fires)
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
		h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "right", Count: uint32(c.focus)})
		if !slices.Equal(log, []string{c.want}) {
			t.Errorf("right over focus %b ran %q, want %q", c.focus, log, c.want)
		}
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
			h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: k})
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
		h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "down"})
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
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "x"})
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
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "down"})
	hidden.Set(false)
	if keys, _ := sentKeys(h.commit()); !slices.Equal(keys, []string{"down"}) {
		t.Fatalf("shown again: keymap = %q", keys)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "down"})
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
		h.tree.Dispatch(driver.Event{Kind: driver.EventKey, Text: "down"})
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
