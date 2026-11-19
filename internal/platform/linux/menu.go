//go:build linux && !android

package linux

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/menutext"
)

// roleActionBase separates a role's action data from a menu item's ID.
const roleActionBase = 1 << 20

// setMenu replaces the application menu and its shortcuts; nil restores
// GTK's own menu bar.
func (w *window) setMenu(entries []driver.MenuEntry) {
	w.chrome.menu = entries
	w.chrome.keys = map[string]menuRef{}
	w.menuKeys(entries)
	w.showMenuBar()
}

// showMenuBar builds the application menu as a GTK menu bar over the
// surface, unless the title bar is hidden or the window is full screen.
func (w *window) showMenuBar() {
	if w.menuBar != 0 {
		// Unparenting drops the bar's only reference.
		gtkWidgetSetVisible(w.menuBar, 0)
		gtkBoxRemove(w.box, w.menuBar)
		w.menuBar = 0
	}
	clear(w.actionRefs)
	c := &w.chrome
	if c.menu == nil || c.titleBar != driver.TitleBarDefault || c.fullScreen || w.transparent ||
		c.menuBar == driver.MenuBarAutoHide && !c.revealed {
		return
	}
	w.actionGroup = gSimpleActionGroupNew()
	menu := w.buildMenu(w.chrome.menu)
	if menu == 0 {
		return
	}
	gtkWidgetInsertActionGroup(w.win, cstr("win"), w.actionGroup)
	gObjectUnref(w.actionGroup)
	bar := gtkPopoverMenuBarNewFromModel(menu)
	gObjectUnref(menu)
	if bar == 0 {
		return
	}
	gtkWidgetSetHalign(bar, 1) // start
	gtkBoxPrepend(w.box, bar)
	gtkWidgetSetVisible(bar, 1)
	w.menuBar = bar
}

// buildMenu returns a GMenuModel of entries.
func (w *window) buildMenu(entries []driver.MenuEntry) uintptr {
	menu := gMenuNew()
	for i := range entries {
		e := &entries[i]
		if e.Separator || !shown(e) {
			continue
		}
		if len(e.Items) > 0 {
			sub := w.buildSubmenu(e.Items)
			if sub != 0 {
				gMenuAppendSubmenu(menu, cstr(menutext.Label(w.language, e, "")), sub)
				gObjectUnref(sub)
			}
			continue
		}
		if item := w.buildItem(e); item != 0 {
			gMenuAppendItem(menu, item)
			gObjectUnref(item)
		}
	}
	return menu
}

// buildSubmenu returns a GMenuModel of a submenu's items, separated into
// sections at the Separator entries as GTK menus show groups.
func (w *window) buildSubmenu(entries []driver.MenuEntry) uintptr {
	sub := gMenuNew()
	section := gMenuNew()
	flush := func() {
		if section == 0 {
			return
		}
		gMenuAppendSection(sub, nil, section)
		gObjectUnref(section)
		section = 0
	}
	for i := range entries {
		e := &entries[i]
		if e.Separator {
			flush()
			section = gMenuNew()
			continue
		}
		if !shown(e) {
			continue
		}
		if len(e.Items) > 0 {
			child := w.buildSubmenu(e.Items)
			if child != 0 {
				gMenuAppendSubmenu(section, cstr(menutext.Label(w.language, e, "")), child)
				gObjectUnref(child)
			}
			continue
		}
		if item := w.buildItem(e); item != 0 {
			gMenuAppendItem(section, item)
			gObjectUnref(item)
		}
	}
	flush()
	return sub
}

// shown reports whether the menu shows e: not a role GTK has no use for, nor a submenu none of whose items it shows, such as a Help holder with no items or a Speech submenu of macOS's roles.
func shown(e *driver.MenuEntry) bool {
	if driver.MenuRoleUnsupported(e.Role, driver.OSLinux, speakAvailable()) {
		return false
	}
	if len(e.Items) == 0 {
		return !e.Role.HoldsMenu()
	}
	for i := range e.Items {
		if !e.Items[i].Separator && shown(&e.Items[i]) {
			return true
		}
	}
	return false
}

// buildItem returns a GMenuItem of e with an action when it has one.
func (w *window) buildItem(e *driver.MenuEntry) uintptr {
	label := menutext.Label(w.language, e, "")
	action := ""
	if e.ID != 0 {
		action = w.addAction(e.ID, e.Role, e.Disabled, e.Checkable, e.Checked, e.Radio)
	} else if e.Role != driver.MenuRoleNone {
		action = w.addAction(0, e.Role, e.Disabled, false, false, false)
	}
	if action == "" {
		return gMenuItemNew(cstr(label), nil)
	}
	prefix := "win."
	if w.building.prefix != "" {
		prefix = w.building.prefix
	}
	item := gMenuItemNew(cstr(label), cstr(prefix+action))
	if item != 0 && e.Radio && e.ID != 0 {
		// A target its action's state may equal makes GTK draw a radio, checked while it does.
		gMenuItemSetAttributeValue(item, cstr("target"), gVariantNewString(cstr(radioTarget)))
	}
	return item
}

// radioTarget is the target of a radio item, and its action's state while it is checked.
const radioTarget = "selected"

// addAction registers a menu item's action on the window and returns its
// name; "" when the item does nothing.
func (w *window) addAction(id uint32, role driver.MenuRole, disabled, checkable, checked, radio bool) string {
	w.menuCounter++
	name := fmt.Sprintf("nsgo.item-%d", w.menuCounter)
	var action uintptr
	switch {
	case radio:
		// A radio's action takes its target as a string parameter, which GTK requires to activate it, and its string state says whether the item is checked. The action sinks the floating variant and owns it.
		state := ""
		if checked {
			state = radioTarget
		}
		action = gSimpleActionNewStateful(cstr(name), cstr("s"), gVariantNewString(cstr(state)))
	case checkable:
		// The action sinks the floating variant and owns it.
		state := gVariantNewBoolean(boolInt(checked))
		action = gSimpleActionNewStateful(cstr(name), nil, state)
	default:
		action = gSimpleActionNew(cstr(name), 0)
	}
	if action == 0 {
		return ""
	}
	if disabled {
		gSimpleActionSetEnabled(action, 0)
	}
	group, refs := w.actionGroup, w.actionRefs
	if w.building.group != 0 {
		group, refs = w.building.group, w.building.refs
	}
	refs[action] = menuRef{id: id, role: role}
	gSignalConnect(action, "activate", cbMenuActivate, w.handle)
	if group != 0 {
		gActionMapAddAction(group, action)
	}
	gObjectUnref(action)
	return name
}

// menuActivateCallback runs the item of a chosen menu action.
func menuActivateCallback(action, parameter, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing {
		return
	}
	if ref, ok := w.actionRefs[action]; ok {
		w.menuChosen(ref)
	} else if ref, ok := w.popupRefs[action]; ok {
		w.menuChosen(ref)
	}
}
