package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// TabBar is the core's part of mobile.TabBar, which apps use: the
// system's tab bar, a native component iOS and Android register. The
// selected tab takes the theme's accent; Android also follows custom palette surface, muted text, indicator and accent-text roles when they differ from the default Material baseline.
type TabBar struct {
	Items     []TabBarItem
	ItemsFunc func() []TabBarItem
	// Selected is the selected tab's index; one past the tabs selects
	// none. The bar shows the user's choice at once, as the system's does.
	Selected func() int
	OnChange func(index int)
	Style    Style
	TestID   string
}

// TabBarItem is one tab of a TabBar. Apps name it mobile.TabBarItem.
type TabBarItem struct {
	Title string
	// Icon is the platform's name for an icon: an SF Symbol on iOS
	// ("house"), and on Android a drawable of the app or, prefixed
	// "android:", of the system ("android:ic_menu_search").
	Icon string
	// SVG is the tab's icon on Android where Icon names no drawable the app or system has, such as an SF Symbol name shared with iOS: fitted into 24 points and tinted by the bar as a drawable is. iOS uses only Icon.
	SVG *SVG
}

// Render implements Node.
func (b TabBar) Render(cx *Scope) Node {
	mobile := func() bool { return cx.Env().mobileBars() }
	return Show{When: mobile, Then: b.native,
		Else: func(*Scope) Node { return View{Style: Style{Display: DisplayNone}, TestID: b.TestID} }}
}

// native is the bar where the platform has one.
func (b TabBar) native(cx *Scope) Node {
	items := func() []TabBarItem {
		if b.ItemsFunc != nil {
			return b.ItemsFunc()
		}
		return b.Items
	}
	return Native{Component: driver.TabBarComponent, Style: b.Style, TestID: b.TestID,
		PropsFunc: func() map[string]any {
			list := []any{}
			for _, it := range items() {
				tab := map[string]any{"title": it.Title}
				if it.Icon != "" {
					tab["icon"] = it.Icon
				}
				if it.SVG != nil && cx.Env().OS() == OSAndroid {
					d := &driver.Drawing{}
					it.SVG.Draw(d, Size{Width: 24, Height: 24}, RGB(0xffffff))
					tab["drawing"] = d
				}
				list = append(list, tab)
			}
			selected := 0
			if b.Selected != nil {
				selected = b.Selected()
			}
			// The accent is always sent. The other palette roles are colors only for a theme that changes the tab bar's roles, and 0, Android's own Material baseline, for NSGo's default theme; they are always sent, so a theme going back to the default takes its overrides off.
			p := cx.Palette()
			props := map[string]any{"items": list, "selectedIndex": float64(selected), "tint": float64(p.Accent),
				"background": float64(0), "inactive": float64(0), "indicator": float64(0), "activeContent": float64(0)}
			base := DefaultTheme().Light
			if cx.Dark() {
				base = DefaultTheme().Dark
			}
			container, hasContainer := p.Color("accentContainer")
			baseContainer, baseHasContainer := base.Color("accentContainer")
			if p.Surface != base.Surface || p.TextMuted != base.TextMuted || p.AccentText != base.AccentText || hasContainer != baseHasContainer || container != baseContainer {
				props["background"] = float64(p.Surface)
				props["inactive"] = float64(p.TextMuted)
				if !hasContainer || container == 0 {
					container = p.Accent.Faded(0.12)
				}
				props["indicator"] = float64(container)
				props["activeContent"] = float64(p.AccentText)
			}
			return props
		},
		OnEvent: func(name string, data any) {
			if name != "change" || b.OnChange == nil {
				return
			}
			switch i := data.(type) {
			case float64:
				b.OnChange(int(i))
			case int:
				b.OnChange(i)
			}
		}}
}
