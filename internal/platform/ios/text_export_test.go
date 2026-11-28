//go:build ios && nsgotest

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TextInkForTest draws the mounted text view with testID and returns the bottoms of the bands of dark ink in it, from its top, with the view's height and the number of lines its layout holds. ok is false while the text has not mounted.
func TextInkForTest(testID string) (ink []float64, height float64, lines int, ok bool) {
	onMainSync(func() {
		for _, v := range runner.views {
			if v == nil || v.layout == nil || v.props.TestID != testID {
				continue
			}
			ok = true
			v.native.Send(sel("layoutIfNeeded"))
			size := objc.Send[apple.CGRect](v.native, sel("bounds")).Size
			ink = inkBaselines(v.native.Send(sel("layer")), size.Width, size.Height)
			height, lines = size.Height, len(v.layout.TextLines())
			return
		}
	})
	return ink, height, lines, ok
}
