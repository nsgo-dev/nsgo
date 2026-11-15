//go:build darwin && !ios

package apple

// holdSpeechSession does nothing on macOS, whose input needs no session.
func holdSpeechSession() (leave func(), err error) { return func() {}, nil }
