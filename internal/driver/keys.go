package driver

import (
	"slices"
	"strings"
)

// HotKeyState is the platform's answer to a global shortcut registration.
type HotKeyState uint8

const (
	// HotKeyPending says the platform has not answered the latest registration.
	HotKeyPending HotKeyState = iota
	// HotKeyRegistered says the shortcut is active.
	HotKeyRegistered
	// HotKeyTaken says another registration owns the combination.
	HotKeyTaken
	// HotKeyUnsupported says the key or the current session cannot provide a global shortcut.
	HotKeyUnsupported
)

func (s HotKeyState) String() string {
	switch s {
	case HotKeyPending:
		return "pending"
	case HotKeyRegistered:
		return "registered"
	case HotKeyTaken:
		return "taken"
	case HotKeyUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// KeyBinding is one binding of the keymap the platform matches key
// presses against, on the main thread.
type KeyBinding struct {
	// Key is normalized, as NormalizeKey writes it.
	Key string
	// InText fires it while a text input or a web view has the keyboard
	// focus.
	InText bool
	// Repeat fires it again on the auto-repeats of a held key; else they
	// are consumed and fire nothing.
	Repeat bool
	// Yields are the focused controls it gives its key to, a subset of what KeyUsedBy says for the key: it takes no press while such a control has the keyboard focus, which an older binding of the key or the control then takes.
	Yields KeyUse
	// ID names the binding: an in-app binding's (ItemKeymap), which EventKey reports when the platform matches it, or a global shortcut's (ItemHotKeys), which EventHotKey reports. The engine never gives an in-app ID to another binding, so a press matched by a keymap that has since changed runs the binding it matched or none.
	ID uint32
}

// KeyUse is a set of the keys a focused view uses itself, by the kind of view.
type KeyUse uint8

const (
	// KeyUsePress is Return and Space, which press a focused pressable view.
	KeyUsePress KeyUse = 1 << iota
	// KeyUseAdjust is the arrows, Page Up, Page Down, Home and End, which move a focused slider or spin button.
	KeyUseAdjust
	// KeyUseSelect is the arrows, Page Up, Page Down, Home and End, which move the selection or the focus in a focused tab bar, radio group, list, menu or tree.
	KeyUseSelect
	// KeyUseText is the keys a focused text input or web view edits with; InText says whether a binding fires over it.
	KeyUseText
)

// KeyUsedBy returns the focused views that use key, a normalized key, themselves: Return and Space a pressable view, the arrows and the paging keys a slider or a selection. Modified keys are none's.
func KeyUsedBy(key string) KeyUse {
	switch key {
	case "enter", "space":
		return KeyUsePress
	case "up", "down", "left", "right", "home", "end", "pageup", "pagedown":
		return KeyUseAdjust | KeyUseSelect
	}
	return 0
}

// FocusKeyUse returns the keys a focused view uses itself, by its kind, its role and whether it takes presses: a text input or web view its editing keys, a pressable view Return and Space, a slider or spin button its adjusting keys, and a tab bar, radio group, list, menu or tree and their items their moving keys. Each backend's focused view and nsgotest's are read the same way.
func FocusKeyUse(kind ViewKind, role Role, pressable bool) KeyUse {
	var use KeyUse
	if kind == KindInput || kind == KindWeb {
		use |= KeyUseText
	}
	if pressable {
		use |= KeyUsePress
	}
	switch role {
	case RoleAdjustable, RoleSpinButton:
		use |= KeyUseAdjust
	case RoleTab, RoleTabList, RoleRadio, RoleRadioGroup, RoleListbox, RoleOption, RoleMenu, RoleMenuItem,
		RoleMenuItemCheckbox, RoleMenuItemRadio, RoleTree, RoleTreeItem:
		use |= KeyUseSelect
	}
	return use
}

// NormalizeKey returns key lowercased with its modifiers in the order mod, ctrl, alt, shift, meta; "cmd" and "command" are meta, "option" is alt, "control" is ctrl and the key "return" is "enter".
func NormalizeKey(key string) string {
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(key, " ", "")), "+")
	name := parts[len(parts)-1]
	if name == "return" {
		name = "enter"
	}
	alias := map[string]string{"cmd": "meta", "command": "meta", "option": "alt", "control": "ctrl"}
	var given []string
	for _, m := range parts[:len(parts)-1] {
		if a, ok := alias[m]; ok {
			m = a
		}
		given = append(given, m)
	}
	var mods []string
	for _, m := range []string{"mod", "ctrl", "alt", "shift", "meta"} {
		if slices.Contains(given, m) {
			mods = append(mods, m)
		}
	}
	return strings.Join(append(mods, name), "+")
}

// FiresOver reports whether the binding takes a press while the view with the keyboard focus uses the keys focus says: not from a text input or web view unless InText, nor from a control whose keys it Yields.
func (b KeyBinding) FiresOver(focus KeyUse) bool {
	return (focus&KeyUseText == 0 || b.InText) && focus&b.Yields == 0
}

// FindKey says what a key press does with keymap: names are the press's normalized names (with "mod" and with the literal modifier), focus the keys the view with the keyboard focus uses itself (FocusKeyUse, KeyUseText alone for a backend that knows only whether a text input or web view has it), and repeat whether it is a held key's auto-repeat. A binding takes no press from a focused text input or web view unless InText, nor from a focused control whose keys it Yields. It returns the ID of the binding that fires, or 0, and whether the press is consumed: a binding fires, or a held key's repeat is taken by a binding that fires once.
func FindKey(keymap []KeyBinding, names []string, focus KeyUse, repeat bool) (id uint32, consumed bool) {
	for i := len(keymap) - 1; i >= 0; i-- {
		b := keymap[i]
		if !slices.Contains(names, b.Key) || !b.FiresOver(focus) {
			continue
		}
		if repeat && !b.Repeat {
			return 0, true
		}
		return b.ID, true
	}
	return 0, false
}

// MenuShortcut finds the enabled item of entries or of their enabled submenus that takes a shortcut of names where the focus is, a role's own where it has none, as the platforms read them: on a Mac every item's, elsewhere all but the editing roles', whose keys the focused field handles itself. The first such item in menu order takes the key, as the platforms' menus find it; a disabled item takes nothing, and on a Mac it keeps the key from the items after it too, as AppKit stops at the first key equivalent that matches. While editing (a text input or web view has the focus), a NotInText item takes nothing and keeps its key from the items after it, since each desktop stops at it: macOS disables it during the key press, and Windows and Linux keep the first item of each shortcut. Linux offers no key pressed without Control, Alt or a logo key to the menu while editing.
func MenuShortcut(entries []MenuEntry, names []string, os OS, editing bool) (MenuEntry, bool) {
	if os == OSLinux && editing && !slices.ContainsFunc(names, commandKey) {
		return MenuEntry{}, false
	}
	e, ok, _ := menuShortcut(entries, names, os == OSMacOS, editing, map[string]bool{})
	return e, ok
}

// commandKey reports whether the normalized key is pressed with Control, Alt or a logo key, Shift not counting.
func commandKey(key string) bool {
	parts := strings.Split(key, "+")
	return slices.ContainsFunc(parts[:len(parts)-1], func(m string) bool { return m == "mod" || m == "ctrl" || m == "alt" || m == "meta" })
}

// menuShortcut is MenuShortcut, also reporting whether an item on a Mac ended the search. Off a Mac, kept holds the shortcuts a NotInText item keeps while editing: Windows and Linux look an item up by the shortcut's own spelling, so another spelling of the press ("ctrl+r" for "mod+r") may still reach a later item.
func menuShortcut(entries []MenuEntry, names []string, mac, editing bool, kept map[string]bool) (found MenuEntry, ok, stopped bool) {
	shortcutOf := func(e MenuEntry) string {
		if e.Shortcut != "" {
			return e.Shortcut
		}
		return NormalizeKey(e.Role.Shortcut(mac))
	}
	for _, e := range entries {
		shortcut := shortcutOf(e)
		has := shortcut != "" && slices.Contains(names, shortcut) && !kept[shortcut]
		switch {
		case e.Separator:
		case len(e.Items) > 0:
			if e.Disabled {
				continue
			}
			if found, ok, stopped := menuShortcut(e.Items, names, mac, editing, kept); ok || stopped {
				return found, ok, stopped
			}
		case e.Disabled:
			if mac && has {
				return MenuEntry{}, false, true
			}
		case !mac && e.Role.EditsText():
			// Windows and Linux leave an editing role's key to the focused field and give the menu none of it.
		case e.NotInText && editing:
			if has && mac {
				return MenuEntry{}, false, true
			}
			if has {
				kept[shortcut] = true
			}
		case has:
			return e, true, false
		}
	}
	return MenuEntry{}, false, false
}
