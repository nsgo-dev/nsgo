package core

import (
	"log"
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Keyboard shortcuts: the surface's table of bindings goes to the
// platform, which matches each key press on the main thread, consumes
// the presses that match and reports them with EventKey.

type keyBinding struct {
	driver.KeyBinding
	owner *Scope
	// within is the scope Confine confined it to, nil for none.
	within *Scope
	// overModals is KeyOptions.OverModals.
	overModals bool
	fn         func()
}

// KeyWhen says when a binding fires.
type KeyWhen uint8

const (
	// KeyAuto fires it unless the view with the keyboard focus uses the key itself: a text input or a web view the keys EditsText reports, a view that takes presses (a button, a menu row, a switch) Return and Space, a slider or spin button the arrows, Page Up, Page Down, Home and End, and a tab bar, radio group, list, menu or tree, or one of their items, the same keys to move in it. Modified keys, such as Shift+Space, are none of these but a text input's.
	KeyAuto KeyWhen = iota
	// KeyAlways fires it wherever the keyboard focus is.
	KeyAlways
	// KeyNotEditing fires it only while no text input or web view has the keyboard focus, and takes its key from any other focused view: a control binds its own keys with it while it has the focus, as a select does Return while its trigger keeps the focus, where KeyAuto would leave them to the control.
	KeyNotEditing
)

// KeyOptions are how a binding fires.
type KeyOptions struct {
	When KeyWhen
	// Repeat fires it again on the auto-repeats of a held key.
	Repeat bool
	// OverModals keeps it firing while a modal its owner's views are not in covers them, for an app-wide shortcut such as a menu's.
	OverModals bool
}

// OnKey runs fn when key is pressed, while the current owner lives and the user can see what it shows. key names modifiers and a key joined by "+": "mod+s" (Command on Apple platforms, Control elsewhere), "ctrl+shift+z", "alt+left", "escape", "enter", "f5", "a". The newest binding of a key wins. It fires once while the key is held, and not while the view with the keyboard focus uses the key itself, a text input its editing keys, a button Return and Space, a slider or a tab bar its arrows (KeyAuto); OnKeyWith says else. A control that binds its own keys while it has the focus binds them with KeyNotEditing, or the focus it holds would keep them from it.
//
// What the owner shows are the views of the nearest owner at or above it that has any. The binding rests while none of them shows: in a Holder's content no place shows, under a DisplayNone view such as a navigation stack's screen below the top, in a dismissed modal. While a modal is shown it rests too if any of them is outside the topmost modal, as the page under a dialog is, unless KeyOptions.OverModals. It fires again once they show, without binding it again.
func (cx *Scope) OnKey(key string, fn func()) { cx.OnKeyWith(key, KeyOptions{}, fn) }

// OnKeyWith is OnKey firing as opts say.
func (cx *Scope) OnKeyWith(key string, opts KeyOptions, fn func()) {
	t := cx.tree()
	if t == nil {
		return
	}
	k := driver.NormalizeKey(key)
	inText := opts.When == KeyAlways || opts.When == KeyAuto && !EditsText(k)
	var yields driver.KeyUse
	if opts.When == KeyAuto {
		yields = driver.KeyUsedBy(k)
	}
	owner := cx.current()
	b := &keyBinding{KeyBinding: driver.KeyBinding{Key: k, InText: inText, Repeat: opts.Repeat, Yields: yields}, owner: owner, within: confinement(owner), overModals: opts.OverModals, fn: fn}
	t.keys = append(t.keys, b)
	t.keysChanged = true
	t.changed = true
	b.owner.addCleanup(func() {
		t.keys = slices.DeleteFunc(t.keys, func(k *keyBinding) bool { return k == b })
		t.keysChanged = true
		t.changed = true
	})
}

type backHandler struct {
	owner, within *Scope
	fn            func() bool
}

// confineKey is where Confine marks an owner.
type confineKey struct{}

// Confine holds what the current owner shows, whoever makes it, to its place in the window, as a browser holds a cross-origin frame. Its key bindings (OnKey) and back handlers (OnBackPress) fire only while the keyboard focus is on a view it owns, itself or through the owners below it: never while the focus is elsewhere or nowhere, so what it shows cannot take the keys or back presses of the views around it; a press they leave goes on to the others, as if they were not there. Its views draw and take input only within its frame, the nearest view around them it does not own, which clips them: its overlays and Fixed views are hosted, sized and placed there rather than in the root view or a modal, and its overlays take back only under the focus rule above. Give it a view of its own, as tsx's sandbox does, or the views beside it in that frame stay within its reach. What a frame cannot hold, and the focus, wait for the user: a Ref's Focus and SetAccessibilityFocus, an input's AutoFocus, DismissKeyboard, a Modal, ShowAlert, PopupMenu and Announce act only while the focus is already in its views or the user's input just landed on one (confinedMay), else they are refused, and the first refusal of each kind logs; a modal so shown stays and hears back wherever the focus is, and back takes it away whether or not it removes the modal. Its navigation stacks pop on back only under the focus rule above. Confine it before it renders. tsx's sandbox confines a bundle's scope.
func (cx *Scope) Confine() {
	o := cx.current()
	if o.values == nil {
		o.values = map[any]any{}
	}
	o.values[confineKey{}] = true
	o.rt.confined = true
}

// confinement is the owner nearest o that Confine marked, nil for none. Apps that never confine a scope skip the walk, which every hoisted view, back press and gated request takes.
func confinement(o *Scope) *Scope {
	if o == nil || !o.rt.confined {
		return nil
	}
	for s := o; s != nil; s = s.parent {
		if _, ok := s.values[confineKey{}]; ok {
			return s
		}
	}
	return nil
}

// focusWithin reports whether the keyboard focus is on a view s owns, itself or through the owners below it; any focus, or none, for a nil s.
func (t *Tree) focusWithin(s *Scope) bool {
	if s == nil {
		return true
	}
	return t.focus != nil && owns(s, t.focus.owner)
}

// owns reports whether s is o or an owner above it.
func owns(s, o *Scope) bool {
	for ; o != nil; o = o.parent {
		if o == s {
			return true
		}
	}
	return false
}

// confinedRefusal is where a confined owner notes that a request of a kind was refused, which logs once.
type confinedRefusal uint8

const (
	refusedFocus confinedRefusal = iota
	refusedPresent
	refusedAnnounce
)

// askedToPresent is what a refused presentation logs: a Modal, ShowAlert or PopupMenu, which the platform shows over the window, where no frame clips it.
const askedToPresent = "asked to present a modal, an alert or a context menu"

// mayMoveFocus reports whether a focus request of owner o, a Ref's Focus or SetAccessibilityFocus or an input's AutoFocus on a view it owns or its DismissKeyboard, may move the keyboard focus (confinedMay): else a confined owner's views could take the focus, and with it the keys, from the views around them whenever they like.
func (t *Tree) mayMoveFocus(o *Scope) bool {
	return t.confinedMay(o, refusedFocus, "asked for the keyboard focus")
}

// confinedMay reports whether owner o may act as the user's own input would let it. One Confine confined may only while the keyboard focus is already in a view it owns, or until the commit after the user's own input (activated) landed on one, as a browser lets a cross-origin frame take the focus or open a popup only on the user's activation. Any other owner always may. The first refusal of each kind of each confined owner logs what it asked.
func (t *Tree) confinedMay(o *Scope, kind confinedRefusal, what string) bool {
	c := confinement(o)
	if c == nil || t.focusWithin(c) || owns(c, t.activated) {
		return true
	}
	if _, ok := c.values[kind]; !ok {
		c.values[kind] = true
		log.Printf("nsgo: a confined view %s while the keyboard focus was elsewhere and the user's input had not landed on the view; refused", what)
	}
	return false
}

// confinedKeys reports whether a binding is confined, whose part in the keymap follows the focus.
func (t *Tree) confinedKeys() bool {
	return slices.ContainsFunc(t.keys, func(b *keyBinding) bool { return b.within != nil })
}

// OnBackPress runs fn when back is pressed (Escape, the back button or gesture, the browser's back) while the current owner lives and the user can see what it shows, as for OnKey: after the overlays and modals, which take it first, and before the navigation stacks, the newest handler first. fn returns true to take it; when none does, the top stack pops, else the platform goes back itself.
func (cx *Scope) OnBackPress(fn func() bool) {
	t := cx.tree()
	if t == nil {
		return
	}
	owner := cx.current()
	b := &backHandler{owner: owner, within: confinement(owner), fn: fn}
	t.backs = append(t.backs, b)
	b.owner.addCleanup(func() { t.backs = slices.DeleteFunc(t.backs, func(o *backHandler) bool { return o == b }) })
}

// ExitApp is the core's part of mobile.ExitApp, which apps use: it
// leaves the app as Android's back button does when nothing takes it.
func (cx *Scope) ExitApp() {
	if t := cx.tree(); t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemBackUnhandled})
		t.changed = true
	}
}

// keymap returns the bindings that may fire where the focus is and as the views show, oldest first: the newest of a key wins.
func (t *Tree) keymap() []driver.KeyBinding {
	out := make([]driver.KeyBinding, 0, len(t.keys))
	if len(t.keys) == 0 {
		return out
	}
	places := t.keyPlaces()
	for _, b := range t.keys {
		if t.keyActive(b, places) {
			out = append(out, b.KeyBinding)
		}
	}
	return out
}

// keyActive reports whether binding b may fire: its owner lives and shows (keyPlaces.shows), above any modal unless it is OverModals, and the focus is within what it is confined to.
func (t *Tree) keyActive(b *keyBinding, places *keyPlaces) bool {
	return !b.owner.disposed && places.shows(b.owner, b.overModals) && t.focusWithin(b.within)
}

// keyPlaces is where the owners of views show, which decides the key bindings and back handlers that take part: read from the views each time a keymap, a key or a back press asks, so it follows a Holder's content in and out of its places, a modal shown or dismissed, and DisplayNone.
type keyPlaces struct {
	of map[*Scope]keyPlace
}

// keyPlace is what an owner's own views are: some live, some shown, some shown under the topmost modal.
type keyPlace uint8

const (
	ownsView keyPlace = 1 << iota
	viewShown
	viewCovered
)

// keyPlaces finds where the owners of the views show. A view shows when the root view holds it, through the places of Holders, and neither it nor a view around it is DisplayNone (a stack's screen below the top, a Suspense's waiting content) or a modal dismissed. A modal covers what shows outside it; nothing covers the topmost modal, nor a Topmost overlay of a host from the root view to it, which shows above that host's modals.
func (t *Tree) keyPlaces() *keyPlaces {
	p := &keyPlaces{of: make(map[*Scope]keyPlace)}
	for _, n := range t.nodes {
		if n != nil && !n.deleted && n.owner != nil {
			p.of[n.owner] |= ownsView
		}
	}
	for n := range t.regions {
		if n.owner != nil {
			p.of[n.owner] |= ownsView
		}
	}
	hosts := []*node{t.view}
	for top := topModal(t.view); top != nil; top = topModal(top) {
		hosts = append(hosts, top)
	}
	top := hosts[len(hosts)-1]
	var walk func(n *node, above bool)
	walk = func(n *node, above bool) {
		if n.deleted || n.style.Display == DisplayNone || n.hoist != nil && n.hoist.dismissed {
			return
		}
		if n == top || n.hoist != nil && n.hoist.topmost() && slices.Contains(hosts, n.hoist.host) {
			above = true
		}
		if n.owner != nil {
			f := p.of[n.owner] | ownsView | viewShown
			if !above {
				f |= viewCovered
			}
			p.of[n.owner] = f
		}
		for _, c := range n.children {
			walk(c, above)
		}
	}
	walk(t.view, len(hosts) == 1)
	return p
}

// shows reports whether what owner o binds takes part: the nearest owner at or above o that has a view, as the owner of a binding often has none of its own (a Show's that binds while a control has the focus), has one shown and, unless overModals, none of them under a modal. An owner with no view around it takes part.
func (p *keyPlaces) shows(o *Scope, overModals bool) bool {
	for s := o; s != nil; s = s.parent {
		if f := p.of[s]; f != 0 {
			return f&viewShown != 0 && (overModals || f&viewCovered == 0)
		}
	}
	return true
}

func (t *Tree) pressKey(key string, focus driver.KeyUse) {
	key = driver.NormalizeKey(key)
	var places *keyPlaces
	for i := len(t.keys) - 1; i >= 0; i-- {
		// A binding is checked again, as the platform may have matched the press by a keymap the focus or the views have since left.
		// The focus the platform matched by passes over the bindings it passed over, so the one it chose runs rather than a newer one of the key that yields to the focused control.
		if b := t.keys[i]; b.Key == key && b.FiresOver(focus) {
			if places == nil {
				places = t.keyPlaces()
			}
			if t.keyActive(b, places) {
				t.within(b.owner, b.fn)
				return
			}
		}
	}
}

// EditsText reports whether a text input with the keyboard focus uses
// the normalized key itself: keys that type, move the caret or delete,
// and select all, cut, copy, paste, undo and redo. A binding of it fires
// in text inputs only with KeyAlways.
func EditsText(key string) bool {
	parts := strings.Split(key, "+")
	name, mods := parts[len(parts)-1], parts[:len(parts)-1]
	command := slices.Contains(mods, "mod") || slices.Contains(mods, "ctrl") || slices.Contains(mods, "meta")
	switch {
	case len(name) > 1 && name[0] == 'f' && name[1] >= '1' && name[1] <= '9':
		return false
	case len(name) == 1:
		return !command || strings.Contains("acvxyz", name)
	}
	return true
}

// KeyLabel is key as menus and hints show it: ⌃⌥⇧⌘K where "mod" is
// Command (apple), else Ctrl+Alt+Shift+K, with "meta" as Win on Windows,
// Meta on the web and Super elsewhere.
func KeyLabel(key string, os OS, apple bool) string {
	parts := strings.Split(driver.NormalizeKey(key), "+")
	name, mods := parts[len(parts)-1], parts[:len(parts)-1]
	has := func(m string) bool { return slices.Contains(mods, m) }
	ctrl, cmd := has("ctrl"), has("meta")
	if has("mod") {
		if apple {
			cmd = true
		} else {
			ctrl = true
		}
	}
	var b strings.Builder
	if apple {
		for _, m := range []struct {
			on    bool
			glyph string
		}{{ctrl, "⌃"}, {has("alt"), "⌥"}, {has("shift"), "⇧"}, {cmd, "⌘"}} {
			if m.on {
				b.WriteString(m.glyph)
			}
		}
	} else {
		meta := map[OS]string{OSWindows: "Win+", OSWeb: "Meta+"}[os]
		if meta == "" {
			meta = "Super+"
		}
		for _, m := range []struct {
			on   bool
			name string
		}{{ctrl, "Ctrl+"}, {has("alt"), "Alt+"}, {has("shift"), "Shift+"}, {cmd, meta}} {
			if m.on {
				b.WriteString(m.name)
			}
		}
	}
	named := map[string][2]string{
		"enter": {"↩", "Enter"}, "escape": {"⎋", "Esc"}, "tab": {"⇥", "Tab"}, "backspace": {"⌫", "Backspace"},
		"delete": {"⌦", "Del"}, "up": {"↑", "↑"}, "down": {"↓", "↓"}, "left": {"←", "←"}, "right": {"→", "→"},
		"home": {"↖", "Home"}, "end": {"↘", "End"}, "pageup": {"⇞", "PgUp"}, "pagedown": {"⇟", "PgDn"},
		"space": {"Space", "Space"},
	}
	switch n, ok := named[name]; {
	case ok && apple:
		b.WriteString(n[0])
	case ok:
		b.WriteString(n[1])
	default:
		b.WriteString(strings.ToUpper(name))
	}
	return b.String()
}

// KeyLabel is key as the platform the app runs on shows it, as
// core.KeyLabel says.
func (cx *Scope) KeyLabel(key string) string {
	env := cx.Env()
	var os OS
	var apple bool
	Untracked(cx, func() bool {
		os, apple = env.OS(), env.AppleKeys()
		return true
	})
	return KeyLabel(key, os, apple)
}

// SetTitle is the core's part of desktop.SetTitle, which apps use.
func (cx *Scope) SetTitle(title string) {
	if t := cx.tree(); t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemTitle, Value: title})
		t.changed = true
	}
}
