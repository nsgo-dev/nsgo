//go:build darwin && !ios

package macos

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

var (
	cgDisplayCopyMode func(uint32) uintptr
	cgDisplayModeRate func(uintptr) float64
	bindDisplayMode   = sync.OnceFunc(func() {
		cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&cgDisplayCopyMode, cg, "CGDisplayCopyDisplayMode")
		purego.RegisterLibFunc(&cgDisplayModeRate, cg, "CGDisplayModeGetRefreshRate")
	})
)

func displayRates(screen objc.ID, number uint32) (rate, maximum float64) {
	if selector := sel("maximumFramesPerSecond"); objc.Send[bool](screen, sel("respondsToSelector:"), selector) {
		maximum = float64(max(0, objc.Send[int](screen, selector)))
	}
	bindDisplayMode()
	if cgDisplayCopyMode != nil && number != 0 {
		if mode := cgDisplayCopyMode(number); mode != 0 {
			rate = cgDisplayModeRate(mode)
			apple.Release(mode)
		}
	}
	if rate <= 0 {
		rate = maximum
	}
	return rate, max(rate, maximum)
}
