package core_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestTabBarTakesTheAccent(t *testing.T) {
	theme := core.DefaultTheme()
	theme.Light.Accent = core.RGB(0xbd481b)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(theme)
		return core.TabBar{Items: []core.TabBarItem{{Title: "Home", Icon: "house"}}, Style: core.Style{Height: core.Pt(49)}, TestID: "bar"}
	}, nsgotest.Environment(driver.Environment{OS: driver.OSIOS}))
	if got := app.Find("id:bar").NativeProps["tint"]; got != float64(core.RGB(0xbd481b)) {
		t.Fatalf("tint = %v, want the theme's accent", got)
	}
}

func TestTabBarSendsCustomPaletteRoles(t *testing.T) {
	theme := core.DefaultTheme()
	theme.Light.Surface, theme.Light.TextMuted, theme.Light.AccentText = core.RGB(0x102030), core.RGB(0x405060), core.RGB(0xffffff)
	theme.Light.SetColor("accentContainer", core.RGB(0x708090))
	var custom *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		custom = core.NewSignal(cx, true)
		cx.SetTheme(theme)
		cx.Effect(func() {
			if !custom.Get() {
				cx.SetTheme(core.DefaultTheme())
			}
		})
		return core.TabBar{Items: []core.TabBarItem{{Title: "Home"}}, Style: core.Style{Height: core.Pt(80)}, TestID: "bar"}
	}, nsgotest.Environment(driver.Environment{OS: driver.OSIOS}))
	app.Settle()
	props := app.Find("id:bar").NativeProps
	for name, want := range map[string]float64{"background": float64(core.RGB(0x102030)), "inactive": float64(core.RGB(0x405060)), "indicator": float64(core.RGB(0x708090)), "activeContent": float64(core.RGB(0xffffff))} {
		if got := props[name]; got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	// Back on the default theme, the overrides go: 0 is Android's Material baseline.
	custom.Set(false)
	app.Settle()
	props = app.Find("id:bar").NativeProps
	for _, name := range []string{"background", "inactive", "indicator", "activeContent"} {
		if got, ok := props[name]; !ok || got != float64(0) {
			t.Errorf("after the default theme came back, %s = %v (sent %v), want 0", name, got, ok)
		}
	}
}

// A web build that runs as iOS or Android has no system tab bar to show.
func TestTabBarShowsNothingSimulated(t *testing.T) {
	for _, os := range []core.OS{core.OSIOS, core.OSAndroid} {
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			return core.TabBar{Items: []core.TabBarItem{{Title: "Home", Icon: "house"}}, TestID: "bar"}
		}, nsgotest.Environment(driver.Environment{OS: os, Idiom: driver.IdiomPhone, Simulated: true}))
		if v := app.FindMounted("id:bar"); v.Component != "" {
			t.Fatalf("%v: mounted the component %q", os, v.Component)
		}
	}
}

func TestTabBarSVGIsAndroidOnlyAndReactive(t *testing.T) {
	for _, os := range []core.OS{core.OSAndroid, core.OSIOS} {
		t.Run(string(os), func(t *testing.T) {
			var set func(*core.SVG)
			app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				icon := core.NewSignal(cx, lucide.House())
				set = icon.Set
				return core.TabBar{TestID: "bar", Style: core.Style{Height: core.Pt(80)}, ItemsFunc: func() []core.TabBarItem {
					return []core.TabBarItem{{Title: "Home", Icon: "house", SVG: icon.Get()}}
				}}
			}, nsgotest.Environment(driver.Environment{OS: os}))
			item := func() map[string]any {
				return app.Find("id:bar").NativeProps["items"].([]any)[0].(map[string]any)
			}
			first, has := item()["drawing"].(*driver.Drawing)
			if has != (os == core.OSAndroid) || item()["icon"] != "house" {
				t.Fatalf("%s tab props = %+v", os, item())
			}
			if has && len(first.Ops) == 0 {
				t.Fatal("SVG produced no drawing")
			}
			app.Post(func() { set(lucide.Search()) })
			app.Settle()
			if os == core.OSAndroid && item()["drawing"] == first {
				t.Fatal("the SVG did not update")
			}
			app.Post(func() { set(nil) })
			app.Settle()
			if _, ok := item()["drawing"]; ok {
				t.Fatal("a removed SVG left a drawing behind")
			}
		})
	}
}
