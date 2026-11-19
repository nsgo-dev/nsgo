package miniprogram

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A mini program runs on its device's OS without its native parts: the environment reports that OS as Simulated, so core mounts a stack page's Header, with Apple's keys on Apple's systems.
func TestDeviceOSIsSimulated(t *testing.T) {
	for _, tc := range []struct {
		os    string
		apple bool
	}{{"ios", true}, {"android", false}, {"macos", true}} {
		var env driver.Environment
		deviceOS(&env, tc.os)
		if env.OS != driver.OS(tc.os) || !env.Simulated || env.AppleKeyboard != tc.apple {
			t.Errorf("%s: OS %q, Simulated %v, AppleKeyboard %v", tc.os, env.OS, env.Simulated, env.AppleKeyboard)
		}
	}
}
