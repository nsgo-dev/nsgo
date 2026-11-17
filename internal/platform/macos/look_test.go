//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// A window made transparent is borderless and clear, still takes the keyboard and Close's Cmd+W and keeps its content's size; made framed again it has its title bar and opaque background back. Off the taskbar it is out of the Window menu and Cmd+`.
func TestTransparentWindow(t *testing.T) {
	onMain(func() {
		loadAppKit()
		pool := objc.ID(apple.Class("NSAutoreleasePool")).Send(selNew)
		defer pool.Send(sel("drain"))
		window := objc.ID(classWindow).Send(selAlloc).Send(sel("initWithContentRect:styleMask:backing:defer:"),
			apple.Rect(0, 0, 160, 100), uint(styleTitled|styleClosable|styleMiniaturizable|styleResizable), uint(2), false)
		window.Send(sel("setReleasedWhenClosed:"), false)
		s := newSurface(Options{Width: 160, Height: 100}, false)
		s.window, s.id = window, 1
		window.Send(sel("setContentView:"), s.native)
		defer func() {
			delete(views, s.native)
			s.native.Send(selRelease)
			window.Send(sel("close"))
			window.Send(selRelease)
		}()
		content := func() apple.CGSize {
			return objc.Send[apple.CGRect](window, sel("contentRectForFrameRect:"), objc.Send[apple.CGRect](window, sel("frame"))).Size
		}
		size := content()

		s.setTransparent(true)
		if mask := objc.Send[uint](window, selStyleMask); mask&(styleTitled|styleClosable) != 0 {
			t.Errorf("a transparent window has style mask %#x; want it borderless", mask)
		}
		if objc.Send[bool](window, sel("isOpaque")) {
			t.Error("a transparent window is opaque")
		}
		if !objc.Send[bool](window, sel("canBecomeKeyWindow")) {
			t.Error("a transparent window cannot take the keyboard")
		}
		closeItem := objc.ID(apple.Class("NSMenuItem")).Send(selAlloc).Send(sel("initWithTitle:action:keyEquivalent:"),
			apple.NSString("Close"), sel("performClose:"), apple.NSString("w"))
		defer closeItem.Send(selRelease)
		if !objc.Send[bool](window, sel("validateMenuItem:"), closeItem) {
			t.Error("a transparent window disables Close and Cmd+W")
		}
		if got := content(); got != size {
			t.Errorf("a transparent window's content is %v; want %v kept", got, size)
		}
		if s.titleBarArea() != (driver.Edges{}) {
			t.Errorf("a transparent window reports a title bar of %v", s.titleBarArea())
		}

		s.setTransparent(false)
		if mask := objc.Send[uint](window, selStyleMask); mask&styleTitled == 0 {
			t.Errorf("a framed window has style mask %#x; want it titled", mask)
		}
		if !objc.Send[bool](window, sel("isOpaque")) {
			t.Error("a framed window is not opaque")
		}
		if got := content(); got != size {
			t.Errorf("a framed window's content is %v; want %v kept", got, size)
		}

		s.setSkipTaskbar(true)
		if !objc.Send[bool](window, sel("isExcludedFromWindowsMenu")) || objc.Send[uint](window, sel("collectionBehavior"))&ignoresCycle == 0 {
			t.Error("a window off the taskbar is in the Window menu or Cmd+`")
		}
		s.setSkipTaskbar(false)
		if objc.Send[bool](window, sel("isExcludedFromWindowsMenu")) || objc.Send[uint](window, sel("collectionBehavior"))&ignoresCycle != 0 {
			t.Error("a window back on the taskbar stays out of the Window menu or Cmd+`")
		}
	})
}
