//go:build android

package crash

// installNativeHandler is a no-op in the Go child process on Android: Go's
// runtime/debug.SetCrashOutput captures fatal Go errors and native signals in
// active-go.log, while the Android Java runtime installs
// Thread.setDefaultUncaughtExceptionHandler (CrashReports.java) to write
// uncaught Java exceptions directly to active-native.log in the same crashes
// directory, which Install promotes on the next launch.
func installNativeHandler(string) {}
