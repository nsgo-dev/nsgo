//go:build darwin && !ios

package apple

import "github.com/nsgo-dev/nsgo/internal/platform/privacy"

// holdSpeechSession does nothing on macOS, whose input needs no session.
func holdSpeechSession() (leave func(), err error) { return func() {}, nil }

// DeclaresUsage reports whether the app macOS asks on behalf of declares key in its Info.plist (privacy.DeclaresUsage).
func DeclaresUsage(key string) bool { return privacy.DeclaresUsage(key) }
