package driver

import "testing"

func TestMenuShortcutEditing(t *testing.T) {
	entries := []MenuEntry{
		{Label: "Refresh", Shortcut: "mod+r", NotInText: true},
		{Label: "Run", Shortcut: "mod+r"},
	}
	for _, os := range []OS{OSMacOS, OSWindows, OSLinux} {
		if _, ok := MenuShortcut(entries, []string{"mod+r", "ctrl+r", "meta+r"}, os, true); ok {
			t.Errorf("%s: a NotInText item let a later item take its shortcut", os)
		}
	}
	for _, os := range []OS{OSWindows, OSLinux} {
		entries := []MenuEntry{{Label: "Refresh", Shortcut: "f5"}}
		_, ok := MenuShortcut(entries, []string{"f5"}, os, true)
		if os == OSLinux && ok {
			t.Errorf("%s: an unmodified shortcut reached the menu while editing", os)
		}
		if os == OSWindows && !ok {
			t.Errorf("%s: an unmodified shortcut did not reach the menu", os)
		}
	}
}
