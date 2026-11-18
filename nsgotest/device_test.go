package nsgotest

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// TestEnvironmentSetsTheDevice: a test's device name and model reach what the app shows.
func TestEnvironmentSetsTheDevice(t *testing.T) {
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.Text{ContentFunc: func() string { return "name:" + cx.Env().DeviceName() }},
			core.Text{ContentFunc: func() string { return "model:" + cx.Env().DeviceModel() }},
		}}
	}, Environment(core.Environment{DeviceName: "Test device", DeviceModel: "Test model"}))
	if got, want := app.Texts(), []string{"name:Test device", "model:Test model"}; !slices.Equal(got, want) {
		t.Fatalf("texts %q, want %q", got, want)
	}
}
