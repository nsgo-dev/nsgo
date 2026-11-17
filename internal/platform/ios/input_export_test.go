//go:build ios && nsgotest

package ios

import "github.com/ebitengine/purego/objc"

// RetainInputForTest keeps an input's UIKit view alive as an outgoing transition can. The first closure lays it out once it is unmounted and reports whether its editor has also been unregistered; the second releases the retained view. Neither closure accesses UIKit off main.
func RetainInputForTest(testID string) (func() bool, func()) {
	var native, editor objc.ID
	onMainSync(func() {
		for _, v := range runner.views {
			if v != nil && v.input != nil && v.props.TestID == testID {
				native, editor = v.native.Send(sel("retain")), v.input.field
				break
			}
		}
	})
	if native == 0 {
		return nil, nil
	}
	return func() bool {
		var gone bool
		onMainSync(func() {
			if views[native] == nil {
				// Deliver the real Objective-C callback after unmount, independent of UIKit's transition timing.
				native.Send(sel("layoutSubviews"))
				gone = views[editor] == nil
			}
		})
		return gone
	}, func() { onMainSync(func() { native.Send(selRelease) }) }
}
