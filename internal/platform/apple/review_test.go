//go:build darwin

package apple

import (
	"testing"

	"github.com/ebitengine/purego/objc"
)

func TestStoreKitReviewControllerLoads(t *testing.T) {
	if !ReviewAvailable() {
		t.Fatal("ReviewAvailable() = false, want true on Darwin")
	}
	cls := loadStoreKitReview()
	if cls == 0 {
		t.Fatal("StoreKit.framework did not expose SKStoreReviewController")
	}
	if !objc.Send[bool](objc.ID(cls), sel("respondsToSelector:"), sel("requestReview")) {
		t.Fatal("SKStoreReviewController does not respond to requestReview")
	}
}
