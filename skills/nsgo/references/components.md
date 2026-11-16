# Choosing components

Use `ui`'s component whenever one exists: it is `base`'s control with NSGo's look, following the theme. Reach for `base` only for a look of your own, and for `core` for primitives. Check the `ui` and `base` packages in the module directory (`go list -m -f '{{.Dir}}' github.com/nsgo-dev/nsgo`) before building a control, and `examples/gallery` for each one's demo.

| Need | Use |
| --- | --- |
| Box, text, image, icon | `core.View`, `core.Text` with `core.Span` runs, `core.Image`, `core.Icon` with icons from `icons/lucide`; `ui.Textf` for text from a signal in a size, weight and color |
| Buttons | `ui.Button` (`ButtonPrimary`, `ButtonSecondary`, `ButtonOutline`, `ButtonGhost`), `ui.IconButton` with a `Label` for screen readers, `ui.ToggleButton`, `ui.ToggleGroup`, `ui.HoldToConfirm`, `ui.Link` |
| Text fields | `ui.Input` (controlled with `Value` and `OnChange`, `Multiline`, `Masked`), `ui.NumberInput`, `ui.OtpInput`, `ui.TagInput`, `ui.FormField` for a label and message |
| Choices | `ui.Switch`, `ui.Checkbox`, `ui.Radio` in a `ui.RadioGroup`, `ui.Select[T]` with `ui.Option[T]`, `ui.Combobox`, `ui.Slider`, `ui.Rating` |
| Dates, times and colors | `ui.DatePicker`, `ui.Calendar`, `ui.TimeField`, `ui.ColorPicker` |
| Scrolling and long data | `ui.ScrollView` or `core.Scroll`; `ui.List` or `core.List` for rows built only in view; `core.NewCollection` behind them |
| Tables, trees and charts | `base/datatable` for large tables, `base.Table` for small ones, `ui.Tree`, `base/plot` for charts |
| Navigation | `base.NewNav` with `ui.NavPage` and `ui.NavHeader` (the system's header on iOS and Android), `ui.Tabs[T]`, `ui.Sidebar`, `ui.SidebarShell`, `ui.TabShell` (a sidebar when wide, a bar when narrow), `mobile.TabBar` for the system tab bar, `ui.Breadcrumbs`, `ui.Pagination`, `ui.Stepper` |
| Overlays | `ui.Dialog`, `ui.AlertDialog`, `cx.ShowAlert` for the system's alert, `ui.Sheet`, `ui.Popover`, `ui.HoverCard`, `ui.Tooltip`, `ui.Toaster` for toasts |
| Menus and commands | `ui.Menu` with `ui.MenuItem` and `ui.Submenu`, `ui.ContextMenu`, `ui.CommandDialog`, `ui.Toolbar` |
| Disclosure and layout helpers | `ui.Accordion`, `ui.Collapsible`, `ui.Carousel`, `ui.Spacer`, `ui.HStack`, `ui.Wrap`, `ui.Settings` and `ui.FormSection` for settings pages |
| Progress and status | `ui.Progress`, `ui.ProgressRing`, `ui.Spinner`, `ui.Skeleton`, `ui.Meter`, `ui.Badge`, `ui.Avatar`, `ui.AvatarGroup` |
| Documents and editors | `base/textview` for streaming Markdown with formulas (`base/formula`) and Mermaid diagrams (`base/diagram`), `base/editor` for code, `ui/richeditor` for rich text, `ui/diff`, `base/terminal` |
| Complex views | `ui/chat` for conversations, `ui/flow` for flow diagrams, `ui/commitgraph`, `base/dock` for dockable panels |
| Web content and media | `core.WebView`, `core.Video`, `core.Canvas` for 2D drawing and shaders |
| Platform services | `cx` methods and `core.Env` (clipboard, storage, secrets, links, pickers, speaking, notifications, haptics, permissions, biometrics, Sign in with Apple, in-app purchases through `cx.Purchase`, OAuth sign-in through `cx.AuthSession` rather than a web view, and `cx.SetWebCookies` to carry a sign-in into web views), `desktop` for windows, trays and menus, `mobile` for phones, `widget` for widgets and Live Activities, `updater` for updates |

## Using them well

- A component's look comes from its `Variant`, a type of its own whose zero value is the default (`ui.ButtonVariant`), and a few take `Size` (`ui.SizeDefault`, small or large). Do not fake a variant with styles over the default.
- Controlled controls take a getter and a setter: `Value: name.Get, OnChange: name.Set`, `Checked: on.Get`, `Open: dialog.Get, OnOpenChange: ...`.
- Use `ui.Slider.MinFunc`, `MaxFunc` and `StepFunc` for changing ranges and steps; `base.Slider.Min`, `Max` and `Step` already take getters. This keeps focus and an active drag mounted.
- Give each control a label screen readers hear (`Label`, or the button's text); icon-only buttons need one.
- `ui.NavPage` and `ui.NavHeader` draw the system's own header and back gesture on phones; do not build a header of views there.
- `ui.Menu` and `ui.Select` open beside their trigger on any device, or as a sheet when their `Presentation` asks; choose it explicitly instead of switching by device.
