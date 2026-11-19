package driver

import "testing"

// TestMenuShortcutEditing holds MenuShortcut to the items the desktop menus choose while a field or web view has the focus (#710): each desktop stops at a NotInText item, Windows and Linux by the shortcut's spelling, and Linux offers no unmodified key to the menu.
func TestMenuShortcutEditing(t *testing.T) {
	cmdR := []string{"mod+r", "meta+r"}
	ctrlR := []string{"mod+r", "ctrl+r"}
	for _, c := range []struct {
		name    string
		os      OS
		entries []MenuEntry
		names   []string
		editing bool
		want    string // the label of the item that takes the key, "" for none
	}{
		{"mac NotInText ends the search", OSMacOS, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "mod+r"}}, cmdR, true, ""},
		{"mac NotInText ends the search for another spelling", OSMacOS, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "meta+r"}}, cmdR, true, ""},
		{"windows NotInText ends the search", OSWindows, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "mod+r"}}, ctrlR, true, ""},
		{"linux NotInText ends the search", OSLinux, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "mod+r"}}, ctrlR, true, ""},
		{"linux NotInText in a submenu ends the search", OSLinux, []MenuEntry{{Label: "View", Items: []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}}}, {Label: "Run", Shortcut: "mod+r"}}, ctrlR, true, ""},
		// Windows and Linux look an item up by its shortcut's spelling, each name of the press in turn.
		{"windows names beat menu order", OSWindows, []MenuEntry{{Label: "Ctrl", Shortcut: "ctrl+r"}, {Label: "Mod", Shortcut: "mod+r"}}, ctrlR, false, "Mod"},
		{"linux names beat menu order", OSLinux, []MenuEntry{{Label: "Ctrl", Shortcut: "ctrl+r"}, {Label: "Mod", Shortcut: "mod+r"}}, ctrlR, false, "Mod"},
		{"windows NotInText keeps only its spelling", OSWindows, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "ctrl+r"}}, ctrlR, true, "Run"},
		{"linux NotInText keeps only its spelling", OSLinux, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "ctrl+r"}}, ctrlR, true, "Run"},
		// Windows and Linux never register an editing role's key, so a NotInText one keeps nothing from later items.
		{"windows NotInText editing role", OSWindows, []MenuEntry{{Label: "Copy", Role: MenuCopy, Shortcut: "mod+c", NotInText: true}, {Label: "Clone", Shortcut: "mod+c"}}, []string{"mod+c", "ctrl+c"}, true, "Clone"},
		{"windows NotInText not editing", OSWindows, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "mod+r"}}, ctrlR, false, "Refresh"},
		{"mac NotInText not editing", OSMacOS, []MenuEntry{{Label: "Refresh", Shortcut: "mod+r", NotInText: true}, {Label: "Run", Shortcut: "mod+r"}}, cmdR, false, "Refresh"},
		{"linux unmodified key while editing", OSLinux, []MenuEntry{{Label: "Reload", Shortcut: "f5"}}, []string{"f5"}, true, ""},
		{"linux shifted key while editing", OSLinux, []MenuEntry{{Label: "Reload", Shortcut: "shift+f5"}}, []string{"shift+f5"}, true, ""},
		{"linux alt key while editing", OSLinux, []MenuEntry{{Label: "Reload", Shortcut: "alt+f5"}}, []string{"alt+f5"}, true, "Reload"},
		{"linux unmodified key not editing", OSLinux, []MenuEntry{{Label: "Reload", Shortcut: "f5"}}, []string{"f5"}, false, "Reload"},
		{"windows unmodified key while editing", OSWindows, []MenuEntry{{Label: "Reload", Shortcut: "f5"}}, []string{"f5"}, true, "Reload"},
		{"mac unmodified key while editing", OSMacOS, []MenuEntry{{Label: "Reload", Shortcut: "f5"}}, []string{"f5"}, true, "Reload"},
	} {
		e, ok := MenuShortcut(c.entries, c.names, c.os, c.editing, true)
		if got := map[bool]string{true: e.Label}[ok]; got != c.want {
			t.Errorf("%s: item %q (found %v), want %q", c.name, got, ok, c.want)
		}
	}
}

func TestMenuShortcutSkipsUnsupportedRoles(t *testing.T) {
	for _, c := range []struct {
		name string
		os   OS
		role MenuRole
	}{
		{"linux macOS role", OSLinux, MenuAbout},
		{"windows speech role", OSWindows, MenuStartSpeaking},
	} {
		entries := []MenuEntry{{Label: "Unsupported", Role: c.role, Shortcut: "mod+r"}, {Label: "Run", Shortcut: "mod+r"}}
		e, ok := MenuShortcut(entries, []string{"mod+r", "ctrl+r"}, c.os, false, false)
		if !ok || e.Label != "Run" {
			t.Errorf("%s: got %+v, found %v, want Run", c.name, e, ok)
		}
	}
}
