//go:build ios && nsgotest

package watchos

import "github.com/nsgo-dev/nsgo/internal/driver"

// AnimatedForTest returns the animated values the view with testID last took, read on the main thread, and whether it has any.
func AnimatedForTest(testID string) (driver.AnimatedValues, bool) {
	var (
		v  driver.AnimatedValues
		ok bool
	)
	onMainSync(func() {
		for _, vw := range s.views {
			if vw != nil && !vw.deleted && vw.props.TestID == testID && vw.animated != nil {
				v, ok = *vw.animated, true
				return
			}
		}
	})
	return v, ok
}
