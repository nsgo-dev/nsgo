//go:build nsgoharmony

package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestHarmonyOSPages opens every demo the HarmonyOS build lists, from its tab's index under the drawn tab bar, and the settings, on the emulator: nsgo test -target harmonyos -run TestHarmonyOSPages -timeout 10m ./examples/gallery.
func TestHarmonyOSPages(t *testing.T) {
	app := nsgotest.Start(t, Root)
	// The device keeps the demo an earlier run left open, which the gallery opens again.
	if d := shell.shownDemo(); d != "" {
		app.Back()
		app.WaitForGone("id:screen-"+d, 5*time.Second)
	}
	for i, tab := range tabs {
		app.Press("id:tab-bar >> role:tab >> nth=" + itoa(i))
		for name := range omitted {
			if app.Exists("id:nav-" + name) {
				t.Fatalf("the %s tab lists %s, which this build leaves out", tab.id, name)
			}
		}
		for _, g := range tab.groups {
			for _, d := range g.demos {
				app.Press("id:nav-" + d.name)
				app.Find("id:screen-" + d.name)
				app.Back()
				app.WaitForGone("id:screen-"+d.name, 5*time.Second)
			}
		}
	}
	app.Press("id:tab-bar >> role:tab >> nth=" + itoa(settingsTab()))
	app.Find("id:settings-page")
}
