---
title: Navigation
description: Native navigation stacks, system headers, modals, menus and tab bars.
group: Core concepts
order: 56
---

## A stack

`base.NewNav` keeps a stack of routes of one type: `Push` adds a page, `Pop` removes the top page, `PopToRoot` returns to the first and `Replace` swaps the top. Its `Stack` renders every route as a page, keeping the pages below mounted with their state while the top one shows. `ui.NavPage` gives each page `ui`'s background and a `ui.NavHeader` with its title, back button and buttons.

watchOS has no system navigation bar, tab bar or switch, so there `ui.NavPage` draws its `ui.NavHeader` above the page, back button included, `ui.TabShell` draws its own tab bar and `ui.SwitchSystem` is `ui`'s own switch.

```go
nav := base.NewNav(cx, "inbox")

return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, depth func() int) base.NavPage {
	if route == "inbox" {
		return ui.NavPage(cx, base.NavPage{Title: "Inbox", Children: []core.Node{
			ui.Button{Text: "Open a message", OnPress: func() { nav.Push("message") }},
		}}, ui.NavHeader{})
	}
	return ui.NavPage(cx, base.NavPage{Title: "Message"}, ui.NavHeader{
		OnBack: func() { nav.Pop(1) }, BackLabel: "Inbox",
		Trailing: []core.Node{ui.IconButton{Icon: lucide.Archive, Label: "Archive", Variant: ui.IconButtonGhost, OnPress: archive}},
	})
})
```

On iOS and Android the platform draws the system's header from the page's title and the header's buttons, with its back button and its transition; on macOS, Windows, Linux and the web `ui` draws it.

Give a page its own `Header` to replace `ui.NavHeader`. On iOS and Android it becomes the system header's title view, in place of `HeaderTitle`, beside the page's `HeaderLeft`, `HeaderRight` and the back button; it takes its content's size there, so give a search field or other growing view a width. Elsewhere it is drawn above the content. `ui.NavHeader.Scroll` still connects an iOS page's scrolling to its header's blur.

`NoPopOnBack` keeps a page when back is pressed, `OnBackPress` hears the press and `NoPopOnSwipe` turns off iOS's swipe back. `cx.OnBackPress(func() bool)` gives the app a handler before the stack, for unsaved changes. `OnDismiss` reports that the user popped the page, and the stack removes it.

## Modals

`ui.Dialog` shows a surface over a dimmed app, a sheet from the bottom on touch screens, and `ui.Sheet` a panel from the bottom. Each closes on Escape and a press on its backdrop unless told not to.

```go
ui.Dialog{
	Open:         dialog.Get,
	OnOpenChange: func(open bool, _ base.Reason) { dialog.Set(open) },
	Title:        "Rename",
	Description:  "The new name shows everywhere.",
	Children:     []core.Node{ui.Input{Value: name.Get, OnChange: name.Set}},
	Footer: []core.Node{
		ui.Button{Text: "Cancel", Variant: ui.ButtonSecondary, OnPress: func() { dialog.Set(false) }},
		ui.Button{Text: "Save", OnPress: save},
	},
}
```

`ui.AlertDialog` asks the user to confirm, and `cx.ShowAlert` shows the system's own alert or action sheet ([Dialog](ui/dialog.md)).

## Menus

`ui.Menu` opens `ui.MenuItem`s from a button: beside it, or as a sheet from the bottom when its `Presentation` asks, on any device. `ui.ContextMenu` does the same for a secondary click or a long press, and `ui.Popover`, `ui.HoverCard` and `ui.Tooltip` show other anchored content.

```go
ui.Menu{Trigger: ui.Button{Text: "Actions", Variant: ui.ButtonSecondary}, Children: []core.Node{
	ui.MenuItem{Text: "Rename", OnSelect: rename},
	ui.MenuItem{Text: "Duplicate", OnSelect: duplicate},
	ui.MenuItem{Text: "Delete", Variant: ui.MenuItemDanger, OnSelect: remove},
}}
```

## Tabs

`ui.Tabs[T]` is a segmented control or a tab list, with an indicator that slides to the chosen option. `ui.Sidebar` holds sections and rows for a desktop sidebar, and `ui.TabShell` keeps top-level sections over a bottom bar at every window width. It uses the system's bar on iOS when every tab has a `SystemIcon`, and on Android when every tab has a `SystemIcon` or an SVG `Icon`, showing the SVG wherever `SystemIcon` names no Android drawable, such as an SF Symbol's name; otherwise it draws the bar with the SVG icons. Set `BarHidden` while a pushed detail page is visible to hide either bar and restore the section's bottom safe area; `TabBarInset` becomes 0 while it is hidden.

`mobile.TabBar` is the system's tab bar on iOS and Android, with its Liquid Glass or Material look; it takes `Items` of `mobile.TabBarItem{Title, Icon, SVG}`, the selected index and `OnChange`. Android follows a custom palette's `Surface`, `TextMuted`, `AccentText` and optional `accentContainer` roles for the bar and its tabs; the default theme keeps Material's baseline colours. Elsewhere it takes no room, so an app shows `ui.Tabs` there. `mobile.IsAvailable(cx)` says whether the bar shows. `Icon` names a platform icon; on Android, `SVG`, such as `lucide.House()`, shows wherever `Icon` names no drawable, tinted by the bar as a drawable is.

```go
mobile.TabBar{
	Items: []mobile.TabBarItem{{Title: "Home", Icon: "house"}, {Title: "Search", Icon: "magnifyingglass"}},
	Selected: func() int { return tab.Get() },
	OnChange: func(i int) { tab.Set(i) },
}
```

## Next

- [Forms](forms.md): what a page of a stack holds.
- [Animation](animation.md): how pages and modals move.
- [Layout](layout.md): the safe area around a header.
