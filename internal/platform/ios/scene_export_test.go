//go:build ios && nsgotest

package ios

import "github.com/ebitengine/purego/objc"

// SceneReconnectForTest disconnects the app's scene and connects it again through the scene delegate's methods, as UIKit does with a scene it releases in the background and brings back as the user returns. It reports whether the surface kept its engine and whether its view shows in a new key window of the scene.
func SceneReconnectForTest() (kept, shown bool) {
	onMainSync(func() {
		old, e := window.Send(sel("retain")), runner.engine
		defer old.Send(selRelease)
		scene := old.Send(sel("windowScene"))
		delegate := scene.Send(sel("delegate"))
		if objc.Send[bool](delegate, sel("respondsToSelector:"), sel("sceneDidDisconnect:")) {
			delegate.Send(sel("sceneDidDisconnect:"), scene)
		}
		delegate.Send(sel("scene:willConnectToSession:options:"), scene, scene.Send(sel("session")), objc.ID(0))
		kept = runner.engine == e
		shown = window != 0 && window != old && runner.uiWindow() == window && objc.Send[bool](window, sel("isKeyWindow"))
	})
	return kept, shown
}
