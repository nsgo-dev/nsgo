package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

func watchEnvironment() core.Environment {
	e := phoneEnv
	e.OS, e.Idiom, e.HeaderHeight = core.OSIOS, core.IdiomWatch, 0
	return e
}

func TestWatchDrawsNavigationHeader(t *testing.T) {
	app, _ := start(t, watchEnvironment(), func(cx *core.Scope) core.Node {
		nav := base.NewNav(cx, "home", "detail")
		return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
			return NavPage(cx, base.NavPage{Title: route, TestID: "page-" + route}, NavHeader{TestID: "header", OnBack: func() { nav.Pop(1) }, BackLabel: "Back"})
		})
	})
	app.Find("id:page-detail >> id:header")
	app.Press("role:button Back")
	app.Find("id:page-home >> id:header")
}

func TestWatchDrawsTabBarAndSwitch(t *testing.T) {
	app, _ := start(t, watchEnvironment(), func(cx *core.Scope) core.Node {
		on := core.NewSignal(cx, false)
		return core.View{Children: []core.Node{
			TabShell{Label: "Sections", Tabs: []ShellTab{{Text: "Home", SystemIcon: "house"}, {Text: "Settings", SystemIcon: "gear"}}},
			Switch{Variant: SwitchSystem, Checked: on.Get, OnChange: on.Set, Label: "Wi-Fi", TestID: "switch"},
		}}
	})
	app.Find("role:tablist Sections")
	if app.FindMounted("id:switch").Component != "" {
		t.Fatal("watchOS mounted a native switch")
	}
}
