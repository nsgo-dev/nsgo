//go:build !ios

package audiosession

// apply does nothing outside iOS, whose systems play every app's audio together with no session to set.
func apply(from, to Level) error { return nil }
