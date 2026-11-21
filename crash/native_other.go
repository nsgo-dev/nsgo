//go:build !darwin && !windows && !linux && !js

package crash

// installNativeHandler relies on Go's runtime/debug.SetCrashOutput on targets
// without a separate platform exception runtime.
func installNativeHandler(string) {}
