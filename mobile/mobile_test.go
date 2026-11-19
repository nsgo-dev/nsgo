package mobile_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/mobile"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// A web build that runs as iOS or Android has none of their components.
func TestIsAvailableSimulated(t *testing.T) {
	for _, os := range []core.OS{core.OSIOS, core.OSAndroid} {
		for _, simulated := range []bool{false, true} {
			var available bool
			nsgotest.Start(t, func(cx *core.Scope) core.Node {
				available = mobile.IsAvailable(cx)
				return core.View{}
			}, nsgotest.Environment(core.Environment{OS: os, Idiom: core.IdiomPhone, Simulated: simulated}))
			if available == simulated {
				t.Errorf("%v, simulated %v: IsAvailable = %v", os, simulated, available)
			}
		}
	}
}

func TestIsUnavailableOnWatch(t *testing.T) {
	var available bool
	nsgotest.Start(t, func(cx *core.Scope) core.Node {
		available = mobile.IsAvailable(cx)
		return core.View{}
	}, nsgotest.Environment(core.Environment{OS: core.OSIOS, Idiom: core.IdiomWatch}))
	if available {
		t.Fatal("watchOS reported mobile system components")
	}
}
