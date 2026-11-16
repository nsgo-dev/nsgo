//go:build darwin && !ios

package apple

// holdSpeakSession does nothing on macOS, whose output needs no session.
func holdSpeakSession() (leave func(), err error) { return func() {}, nil }
