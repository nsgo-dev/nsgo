//go:build linux && !android

package crash

// installNativeHandler is a no-op on Linux: Go's runtime/debug.SetCrashOutput
// already writes both Go runtime fatal throws and fatal synchronous signals
// (SIGSEGV, SIGBUS, SIGILL, SIGFPE, SIGABRT) in an async-signal-safe way
// without replacing Go's signal stack.
func installNativeHandler(string) {}
