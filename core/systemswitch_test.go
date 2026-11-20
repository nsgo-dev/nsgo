package core_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestSystemSwitchFollowsTheApp(t *testing.T) {
	for _, os := range []driver.OS{driver.OSMacOS, driver.OSIOS, driver.OSAndroid} {
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			on := core.NewSignal(cx, false)
			return core.SystemSwitch{Checked: on.Get, OnChange: on.Set, Size: "small", Label: "Wi-Fi", TestID: "switch"}
		}, nsgotest.Environment(driver.Environment{OS: os}))
		v := app.FindMounted("id:switch")
		if v.Component != driver.SwitchComponent || v.NativeProps["checked"] != false || v.NativeProps["size"] != "small" {
			t.Fatalf("%v: component %q, props %v", os, v.Component, v.NativeProps)
		}
		app.Emit("id:switch", "change", true)
		if got := app.FindMounted("id:switch").NativeProps["checked"]; got != true {
			t.Fatalf("%v: checked = %v after the user turned it on", os, got)
		}
	}
}

func TestSystemSwitchShowsNothingElsewhere(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.SystemSwitch{TestID: "switch"}
	}, nsgotest.Environment(driver.Environment{OS: driver.OSWindows}))
	if v := app.FindMounted("id:switch"); v.Component != "" {
		t.Fatalf("Windows mounted the component %q", v.Component)
	}
}

func TestSystemSwitchShowsNothingOnWatch(t *testing.T) {
	available := true
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		available = core.SystemSwitchAvailable(cx)
		return core.SystemSwitch{TestID: "switch"}
	}, nsgotest.Environment(driver.Environment{OS: driver.OSIOS, Idiom: driver.IdiomWatch}))
	if available || app.FindMounted("id:switch").Component != "" {
		t.Fatalf("watchOS: available %v, mounted component %q", available, app.FindMounted("id:switch").Component)
	}
}

// A web build that runs as macOS or iOS has no UISwitch or NSSwitch to show.
func TestSystemSwitchShowsNothingSimulated(t *testing.T) {
	for _, os := range []driver.OS{driver.OSMacOS, driver.OSIOS} {
		available := true
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			available = core.SystemSwitchAvailable(cx)
			return core.SystemSwitch{TestID: "switch"}
		}, nsgotest.Environment(driver.Environment{OS: os, Simulated: true}))
		if v := app.FindMounted("id:switch"); available || v.Component != "" {
			t.Fatalf("%v: available %v, mounted the component %q", os, available, v.Component)
		}
	}
}
