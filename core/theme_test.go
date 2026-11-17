package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestDefaultTextColorFollowsPalette(t *testing.T) {
	theme := DefaultTheme()
	theme.Light.Text, theme.Dark.Text = RGB(0x123456), RGB(0xfedcba)
	svg := MustParseSVG([]byte(`<svg viewBox="0 0 4 4"><rect width="4" height="4" fill="currentColor"/></svg>`))
	var app, side *Scope
	var explicit *Signal[Color]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		app = cx
		cx.SetTheme(theme)
		explicit = NewSignal(cx, RGB(0x345678))
		pair := func(prefix string) Node {
			return View{Style: Style{Direction: Row}, Children: []Node{
				Text{TestID: prefix + "text", Content: "text", Spans: []Span{{Content: "span"}}, Style: Style{Background: CurrentColor}},
				Icon{TestID: prefix + "icon", SVG: svg},
				Input{TestID: prefix + "input", Value: func() string { return "input" }},
			}}
		}
		return View{Children: []Node{
			pair("app-"),
			Func(func(cx *Scope) Node {
				side = cx
				cx.SetTheme(nil)
				cx.SetSubtreeAppearance(AppearanceLight)
				return pair("side-")
			}),
			View{StyleFunc: func() Style { return Style{Color: explicit.Get()} }, Children: []Node{pair("explicit-")}},
		}}
	})
	check := func(prefix string, want Color, batch *driver.MountBatch) {
		t.Helper()
		text, icon, input := h.find(prefix+"text"), h.find(prefix+"icon"), h.find(prefix+"input")
		if text.text.text.Style.Color != want || text.text.text.Spans[0].Style.Color != want || input.input.config.Style.Color != want {
			t.Errorf("%s text/span/input = %#x/%#x/%#x, want %#x", prefix, text.text.text.Style.Color, text.text.text.Spans[0].Style.Color, input.input.config.Style.Color, want)
		}
		if text.sent.viewStyle.Background != want {
			t.Errorf("%s currentColor background = %#x, want %#x", prefix, text.sent.viewStyle.Background, want)
		}
		if icon.icon.color.GetUntracked() != want {
			t.Errorf("%s icon color = %#x, want %#x", prefix, icon.icon.color.GetUntracked(), want)
		}
		if batch == nil {
			return
		}
		for _, item := range batch.Items {
			if item.Kind == driver.ItemCanvas && item.ID == icon.id {
				for _, op := range item.Drawing.Ops {
					if op.Kind == DrawFillColor && op.Color == want {
						return
					}
				}
			}
		}
		t.Errorf("%s icon did not send a drawing in %#x", prefix, want)
	}
	b := h.commit()
	check("app-", theme.Light.Text, b)
	check("side-", theme.Light.Text, b)
	check("explicit-", explicit.GetUntracked(), b)
	textID, iconID := h.find("app-text").id, h.find("app-icon").id

	h.tree.SetEnvironment(Environment{Dark: true})
	b = h.commit()
	check("app-", theme.Dark.Text, b)
	check("side-", theme.Light.Text, nil)
	check("explicit-", explicit.GetUntracked(), nil)

	app.SetAppearance(AppearanceLight)
	b = h.commit()
	check("app-", theme.Light.Text, b)
	app.SetAppearance(AppearanceSystem)
	h.commit()

	side.SetSubtreeAppearance(AppearanceDark)
	b = h.commit()
	check("side-", theme.Dark.Text, b)
	replacement := DefaultTheme()
	replacement.Dark.Text = RGB(0xeeddcc)
	app.SetTheme(replacement)
	b = h.commit()
	check("app-", replacement.Dark.Text, b)
	check("side-", replacement.Dark.Text, b)
	check("explicit-", explicit.GetUntracked(), nil)
	sideTheme := DefaultTheme()
	sideTheme.Dark.Text = RGB(0xddeeff)
	side.SetTheme(sideTheme)
	b = h.commit()
	check("side-", sideTheme.Dark.Text, b)
	check("app-", replacement.Dark.Text, nil)
	side.SetTheme(nil)
	b = h.commit()
	check("side-", replacement.Dark.Text, b)

	h.tree.SetTokens(Tokens{Values: map[string]string{"text": "#112233"}, Dark: map[string]string{"text": "#ccbbaa"}})
	b = h.commit()
	check("app-", RGB(0xccbbaa), b)
	check("side-", RGB(0xccbbaa), b)
	check("explicit-", explicit.GetUntracked(), nil)
	explicit.Set(Transparent)
	b = h.commit()
	check("explicit-", Transparent, b)
	explicit.Set(0)
	b = h.commit()
	check("explicit-", RGB(0xccbbaa), b)
	h.tree.SetTokens(Tokens{})
	b = h.commit()
	check("app-", replacement.Dark.Text, b)
	check("explicit-", replacement.Dark.Text, b)
	if h.find("app-text").id != textID || h.find("app-icon").id != iconID {
		t.Fatal("a palette change rebuilt the views")
	}
}

func TestDefaultTextColorBindingLifetime(t *testing.T) {
	var open *Signal[bool]
	var owner *Scope
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		open = NewSignal(cx, true)
		return Show{When: open.Get, Then: func(cx *Scope) Node {
			owner = cx
			children := make([]Node, 64)
			for i := range children {
				children[i] = Text{Content: "text"}
			}
			return View{Children: children}
		}}
	})
	h.commit()
	binding := h.tree.textColors[owner]
	if binding == nil || len(binding.nodes) != 65 {
		t.Fatal("views did not share their owner's palette binding")
	}
	open.Set(false)
	h.commit()
	if len(binding.nodes) != 0 || h.tree.textColors[owner] != nil {
		t.Fatal("unmounted views or their owner retained the palette binding")
	}
	h.tree.SetAppearance(AppearanceDark)
	h.commit()
	open.Set(true)
	h.commit()
	if b := h.tree.textColors[owner]; b == nil || b.color != DefaultTheme().Dark.Text {
		t.Fatal("remounted views did not bind the current appearance")
	}
}

func TestThemeSubtrees(t *testing.T) {
	side := DefaultTheme()
	side.Light.Background, side.Dark.Background = RGB(0x111111), RGB(0x222222)
	var appBg, sideBg, lightBg Color
	var forced *Signal[Appearance]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		forced = NewSignal(cx, AppearanceDark)
		return View{TestID: "app", StyleFunc: func() Style { appBg = cx.Palette().Background; return Style{} },
			Children: []Node{Func(func(cx *Scope) Node {
				cx.SetTheme(side)
				return View{TestID: "side", StyleFunc: func() Style { sideBg = cx.Palette().Background; return Style{} },
					Children: []Node{Func(func(cx *Scope) Node {
						cx.SetSubtreeAppearance(forced.GetUntracked())
						cx.Effect(func() { cx.SetSubtreeAppearance(forced.Get()) })
						return View{TestID: "forced", StyleFunc: func() Style { lightBg = cx.Palette().Background; return Style{} }}
					})}}
			})}}
	})
	h.commit()
	def := DefaultTheme()
	if appBg != def.Light.Background || sideBg != RGB(0x111111) || lightBg != RGB(0x222222) {
		t.Fatalf("backgrounds = %v %v %v", appBg, sideBg, lightBg)
	}
	if a := h.find("forced").appearance; a != AppearanceDark {
		t.Fatalf("forced view's appearance = %v", a)
	}

	// The app turns dark; the forced subtree goes light when asked.
	h.tree.SetEnvironment(Environment{Dark: true})
	forced.Set(AppearanceLight)
	h.drain()
	b := h.commit()
	if appBg != def.Dark.Background || sideBg != RGB(0x222222) || lightBg != RGB(0x111111) {
		t.Fatalf("dark backgrounds = %v %v %v", appBg, sideBg, lightBg)
	}
	id := h.find("forced").id
	sent := false
	for _, it := range b.Items {
		if it.Kind == driver.ItemViewProps && it.ID == id {
			sent = it.ViewProps.Appearance == AppearanceLight
		}
	}
	if !sent || h.find("side").appearance != AppearanceSystem {
		t.Fatalf("props sent = %v, side = %v", sent, h.find("side").appearance)
	}
}

func TestParseTheme(t *testing.T) {
	th, err := ParseTheme([]byte("radius: 12\nlight:\n  accent: \"#112233\"\n  inset: \"#445566\"\ndark:\n  text: white\n"))
	if err != nil {
		t.Fatal(err)
	}
	if th.Radius != 12 || th.Light.Accent != RGB(0x112233) || th.Light.Colors["inset"] != RGB(0x445566) ||
		th.Dark.Text != RGB(0xffffff) || th.Dark.Accent != DefaultTheme().Dark.Accent {
		t.Fatalf("theme = %+v", th)
	}
	if _, err := ParseTheme([]byte(`{"light": {"accent": "nope"}}`)); err == nil {
		t.Fatal("a bad color parsed")
	}
}

// TestShadcnThemesNameTokens reads a shadcn/ui theme file, as NSH's apps
// name theirs, and classes name its colors.
func TestShadcnThemesNameTokens(t *testing.T) {
	th, err := ParseTheme([]byte(`{"cssVars": {
		"theme": {"radius": "12px", "brand-fill": "#c8f26d"},
		"light": {"brand": "#4f6e0c", "muted-foreground": "#5f636a", "ring": "var(--brand)",
			"brand-tint": "color-mix(in srgb, var(--brand) 10%, transparent)"},
		"dark": {"brand": "#c8f26d"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if th.Radius != 12 || th.Tokens["radius"] != "12px" {
		t.Errorf("radius %v, tokens %v", th.Radius, th.Tokens)
	}
	if c, _ := th.Light.Color("ring"); c != RGB(0x4f6e0c) {
		t.Errorf("light ring %#x", c)
	}
	if c, _ := th.Dark.Color("ring"); c != RGB(0xc8f26d) {
		t.Errorf("dark ring follows dark's brand: %#x", c)
	}
	if c, _ := th.Light.Color("brand-tint"); c != RGBA(0x4f6e0c, 0.1) {
		t.Errorf("brand-tint %#x", c)
	}
	env := classEnv{palette: &th.Light}
	s, _, _ := th.tailwind().parseFor("bg-brand text-muted-foreground/50 border-(--brand-fill)", th, hostColors{}).resolve(&env)
	if s.Background != RGB(0x4f6e0c) || s.Color != RGBA(0x5f636a, 0.5) || s.BorderColor != RGB(0xc8f26d) {
		t.Errorf("classes %#x %#x %#x", s.Background, s.Color, s.BorderColor)
	}
}
