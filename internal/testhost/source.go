package testhost

import "runtime"

// RequireSource skips a check that reads repository sources in a browser or
// native test host, where the test package carries no checkout.
func RequireSource(t interface {
	Helper()
	Skip(...any)
}) {
	t.Helper()
	if runtime.GOOS == "js" || Current() != nil {
		t.Skip("reads the package's source on the development machine")
	}
}
