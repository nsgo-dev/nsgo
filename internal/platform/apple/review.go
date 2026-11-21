//go:build darwin

package apple

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const storeKitFrameworkPath = "/System/Library/Frameworks/StoreKit.framework/StoreKit"

// loadStoreKitReview loads StoreKit.framework lazily on first use and returns
// SKStoreReviewController, or 0 if the framework or class is unavailable.
var loadStoreKitReview = sync.OnceValue(func() objc.Class {
	if _, err := purego.Dlopen(storeKitFrameworkPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return 0
	}
	return Class("SKStoreReviewController")
})

// ReviewAvailable reports whether StoreKit's in-app review prompt is available
// on macOS and iOS. StoreKit loads lazily on the first RequestReview call so
// apps that never ask pay nothing at startup.
func ReviewAvailable() bool { return true }

// RequestReview asks StoreKit to show an in-app rating prompt on the main
// thread. When scene is non-zero and SKStoreReviewController responds to
// requestReviewInScene:, it calls [SKStoreReviewController requestReviewInScene:scene];
// otherwise it falls back to [SKStoreReviewController requestReview]. If
// StoreKit or the selectors are unavailable, it is a silent no-op.
func RequestReview(scene objc.ID) {
	if thread := Class("NSThread"); thread != 0 && !objc.Send[bool](objc.ID(thread), sel("isMainThread")) {
		if scene != 0 {
			scene.Send(sel("retain"))
		}
		OnMain(func() {
			requestReviewOnMain(scene)
			if scene != 0 {
				scene.Send(selRelease)
			}
		})
		return
	}
	requestReviewOnMain(scene)
}

func requestReviewOnMain(scene objc.ID) {
	cls := loadStoreKitReview()
	if cls == 0 {
		return
	}
	controller := objc.ID(cls)
	if scene != 0 && objc.Send[bool](controller, sel("respondsToSelector:"), sel("requestReviewInScene:")) {
		controller.Send(sel("requestReviewInScene:"), scene)
		return
	}
	if objc.Send[bool](controller, sel("respondsToSelector:"), sel("requestReview")) {
		controller.Send(sel("requestReview"))
	}
}
