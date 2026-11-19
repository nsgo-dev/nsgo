package miniprogram

import "github.com/nsgo-dev/nsgo/internal/driver"

// deviceOS sets the OS the runtime reports, the device's, as a mini program has it: Simulated, as none of that OS's native parts are there, so the app's own views stand in for its system header and tab bar and a stack's pages show their Header; and Apple's keys on iOS and macOS, which Simulated alone would turn off.
func deviceOS(env *driver.Environment, os string) {
	env.OS = driver.OS(os)
	env.Simulated = true
	env.AppleKeyboard = env.OS == driver.OSIOS || env.OS == driver.OSMacOS
}
