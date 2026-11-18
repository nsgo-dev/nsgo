package testhost

import "runtime"

// RequireSource skips a check that reads the package's source files or compiles them with a JDK when the test runs in a browser or under a backend's test host: the browser and a device carry no checkout, and a desktop target runs the test binary outside the package's directory.
func RequireSource(t interface {
	Helper()
	Skip(...any)
}) {
	t.Helper()
	if runtime.GOOS == "js" || Current() != nil {
		t.Skip("reads the package's source on the development machine")
	}
}
