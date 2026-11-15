//go:build !ios

package audiosession

// applySession does nothing outside iOS, whose systems play every app's audio together with no session to set.
func applySession(from, to Level) error { return nil }
