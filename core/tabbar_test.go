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
