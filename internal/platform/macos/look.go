//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// A transparent window is a borderless NsgoWindow, which the class lets take the keyboard, with a clear background and no opaque content behind the app's views, so the desktop shows wherever they draw nothing. AppKit casts its shadow from what the window draws but keeps it until told otherwise, so a transparent window's shadow is recomputed after each batch, as a pet's pose changes; motion between batches, such as a running animation, catches up at the next one. Leaving transparency puts the title bar back, keeping the content's size and where it is. A window off the taskbar is out of the Window menu and Cmd+`; the Dock is the app's activation policy (setDockVisible).

var selSetHasShadow = sel("setHasShadow:")

// applyBackground makes the window clear behind the app's views while it is transparent or shows a material, else opaque in AppKit's window color.
func (s *surface) applyBackground() {
	clear := s.transparent || s.effect != 0
	color := sel("windowBackgroundColor")
	if clear {
		color = selClearColor
	}
	s.window.Send(selSetOpaque, !clear)
	s.window.Send(selSetBackgroundColor, objc.ID(apple.Class("NSColor")).Send(color))
}

// setTransparent makes a window borderless and clear, or gives it back its title bar and background.
func (s *surface) setTransparent(on bool) {
	w := s.window
	if w == 0 || s.kind != driver.SurfaceWindow || on == s.transparent {
		return
	}
	s.transparent = on
	content := objc.Send[apple.CGRect](w, sel("contentRectForFrameRect:"), objc.Send[apple.CGRect](w, sel("frame")))
	mask := objc.Send[uint](w, selStyleMask)
	if on {
		mask &^= styleTitled | styleClosable | maskFullSize
	} else {
		mask |= styleTitled | styleClosable
	}
	w.Send(sel("setStyleMask:"), mask)
	w.Send(sel("setFrame:display:"), objc.Send[apple.CGRect](w, sel("frameRectForContentRect:"), content), true)
	s.applyBackground()
	if !on {
		// The title bar the app hid, and its traffic lights, as it left them.
		s.applyTitleBar()
		return
	}
	s.refresh()
}

// reshadow has AppKit cast a transparent window's shadow again from what the window now draws.
func (s *surface) reshadow() {
	if s.transparent && s.window != 0 && objc.Send[bool](s.window, sel("hasShadow")) {
		s.window.Send(sel("invalidateShadow"))
	}
}

// setSkipTaskbar keeps the window out of the Window menu and Cmd+`, or puts it back.
func (s *surface) setSkipTaskbar(on bool) {
	w := s.window
	s.skipTaskbar = on
	w.Send(sel("setExcludedFromWindowsMenu:"), on)
	behavior := objc.Send[uint](w, sel("collectionBehavior"))
	if on {
		behavior |= ignoresCycle
	} else if objc.Send[int](w, sel("level")) >= levelNormal {
		// A window at the desktop's level stays out of Cmd+` (setLevel).
		behavior &^= ignoresCycle
	}
	w.Send(sel("setCollectionBehavior:"), behavior)
}

// ownWindow reports whether the surface shows in a window of its own, which turns transparent and leaves the Window menu: not a view a host embeds, the app's own surface or a tray icon's, which show in none, or a panel.
func (s *surface) ownWindow() bool {
	return !s.embedded && s.window != 0 && s.kind == driver.SurfaceWindow
}
