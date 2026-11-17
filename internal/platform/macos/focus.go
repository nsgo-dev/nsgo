//go:build darwin && !ios

package macos

import (
	"slices"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Keyboard focus for pressable views, as NSH's macOS backend gives it: a
// view that takes presses or is Focusable is in AppKit's key view loop
// with the text fields, unless it is disabled, hidden, NoFocus or under a
// newer modal;
// a click presses it without focusing it, and Return or Space press it
// once while it has the focus. A modal remembers what had the focus as it
// came and gives it back when it goes, if the focus fell to the window.

// focusMethods are the views' first responder methods.
func focusMethods() []objc.MethodDef {
	return []objc.MethodDef{
		// Only the view itself takes the focus: the holder of its children (an effect's or a clip's container) is no second Tab stop.
		{Cmd: sel("acceptsFirstResponder"), Fn: func(self objc.ID, _ objc.SEL) bool {
			v := views[self]
			return v != nil && v.native == self && v.focusable() && (v.surface.focusRequest || !v.surface.clicking())
		}},
		{Cmd: sel("canBecomeKeyView"), Fn: func(self objc.ID, _ objc.SEL) bool {
			v := views[self]
			return v != nil && v.native == self && v.focusable()
		}},
		{Cmd: sel("becomeFirstResponder"), Fn: func(self objc.ID, cmd objc.SEL) bool {
			if v := views[self]; v != nil {
				v.dispatch(driver.EventFocus)
			}
			return true
		}},
		{Cmd: sel("resignFirstResponder"), Fn: func(self objc.ID, cmd objc.SEL) bool {
			if v := views[self]; v != nil {
				v.dispatch(driver.EventBlur)
			}
			return true
		}},
		{Cmd: sel("keyDown:"), Fn: func(self objc.ID, cmd objc.SEL, event objc.ID) {
			v := views[self]
			const tab, shift = 48, 1 << 17
			if v != nil && objc.Send[uint16](event, sel("keyCode")) == tab {
				// The surface around takes the keys that reach it; Tab moves
				// the focus here.
				if objc.Send[uint](event, sel("modifierFlags"))&shift != 0 {
					v.surface.window.Send(sel("selectPreviousKeyView:"), self)
				} else {
					v.surface.window.Send(sel("selectNextKeyView:"), self)
				}
				return
			}
			if v == nil || !v.pressable() || !activates(event) {
				apple.Super(self, cmd, event)
				return
			}
			if !objc.Send[bool](event, sel("isARepeat")) {
				mods := apple.Modifiers(objc.Send[uint](event, sel("modifierFlags")))
				for _, k := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
					v.report(driver.Event{Kind: k, Index: -1, Count: 1, Modifiers: mods})
				}
			}
		}},
		{Cmd: sel("keyUp:"), Fn: func(self objc.ID, cmd objc.SEL, event objc.ID) {
			if v := views[self]; v == nil || !v.pressable() || !activates(event) {
				apple.Super(self, cmd, event)
			}
		}},
	}
}

// activates reports whether a key event is Return, the keypad's Enter or
// Space. The keymap has already had the modified key.
func activates(event objc.ID) bool {
	code := objc.Send[uint16](event, sel("keyCode"))
	return code == 36 || code == 76 || code == 49
}

// focusable reports whether v may take the keyboard focus.
func (v *view) focusable() bool {
	return v.kind != driver.KindInput && v.props.FocusStop(v.events) && v.inFocusScope()
}

func (v *view) inFocusScope() bool {
	if v.native == 0 || objc.Send[bool](v.native, sel("isHiddenOrHasHiddenAncestor")) || v.native.Send(selWindow) == 0 {
		return false
	}
	if n := len(v.surface.modals); n > 0 {
		top := v.surface.modals[n-1]
		return objc.Send[bool](v.native, sel("isDescendantOf:"), top.native)
	}
	return true
}

// tabFromSurface enters AppKit's key view loop from the app's root. The content view can hold a text selection but is no Tab stop; clearing it to the window and selecting next would select that content view again.
func (s *surface) tabFromSurface(backward bool) {
	root := s.views[s.root]
	if root == nil {
		return
	}
	// AppKit may not yet have recalculated a background window's loop.
	s.window.Send(sel("recalculateKeyViewLoop"))
	if !backward && root.focusable() {
		s.window.Send(sel("makeFirstResponder:"), root.native)
		return
	}
	action := "selectKeyViewFollowingView:"
	if backward {
		action = "selectKeyViewPrecedingView:"
	}
	s.window.Send(sel(action), root.native)
}

// focus moves the first responder to a field or a keyboard stop. Clearing the field editor first also lets a command issued by a mouse press move focus: acceptsFirstResponder otherwise keeps that click in the editor.
func (v *view) focus() {
	if !v.inFocusScope() {
		return
	}
	if v.input != nil {
		if !v.props.Disabled && !v.input.fs.Config.Disabled {
			v.input.focus()
		}
		return
	}
	if !v.focusable() {
		return
	}
	w := v.native.Send(selWindow)
	if w.Send(sel("firstResponder")) == v.native {
		return
	}
	w.Send(selMakeFirstResponer, objc.ID(0))
	v.surface.focusRequest = true
	defer func() { v.surface.focusRequest = false }()
	w.Send(selMakeFirstResponer, v.native)
}

// blur takes focus only from this view, leaving another responder alone.
func (v *view) blur() {
	if v.input != nil {
		v.input.unfocus()
	} else if w := v.native.Send(selWindow); w != 0 && w.Send(sel("firstResponder")) == v.native {
		w.Send(selMakeFirstResponer, objc.ID(0))
	}
}

// clicking reports whether the window is handling a mouse press, which
// presses a view without focusing it.
func (s *surface) clicking() bool {
	const leftDown, rightDown, otherDown = 1, 3, 25
	e := objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication")).Send(sel("currentEvent"))
	if e == 0 {
		return false
	}
	t := objc.Send[uint](e, sel("type"))
	return t == leftDown || t == rightDown || t == otherDown
}

// focusKeyUse returns the keys the view with the keyboard focus uses itself, which the keymap's bindings that yield them leave it (driver.FindKey): a field's or a page's editing keys, a pressable view's Return and Space, a slider's or a tab bar's arrows.
func (s *surface) focusKeyUse() driver.KeyUse {
	if s.editing() {
		return driver.KeyUseText
	}
	v := views[s.window.Send(sel("firstResponder"))]
	if v == nil || v.surface != s {
		return 0
	}
	return driver.FocusKeyUse(v.kind, v.props.Role, v.pressable())
}

// observeWebFocus follows NSWindow's KVO-compliant firstResponder, including WebKit's descendants and windows belonging to an embedding host.
func (s *surface) observeWebFocus() {
	if s.stopWebFocus != nil {
		s.stopWebFocus()
		s.stopWebFocus = nil
	}
	w := s.window
	if w == 0 {
		s.reportWebFocus()
		return
	}
	key := apple.NSString("firstResponder").Send(sel("retain"))
	context := uintptr(s.native)
	w.Send(sel("addObserver:forKeyPath:options:context:"), appObserver, key, uint(0), context)
	s.stopWebFocus = func() {
		w.Send(sel("removeObserver:forKeyPath:context:"), appObserver, key, context)
		key.Send(selRelease)
	}
	s.reportWebFocus()
}

func (s *surface) reportWebFocus() {
	var focused *view
	for n := s.window.Send(sel("firstResponder")); n != 0 && objc.Send[bool](n, sel("isKindOfClass:"), apple.Class("NSView")); n = n.Send(selSuperview) {
		if v := views[n]; v != nil && v.surface == s && v.kind == driver.KindWeb {
			focused = v
			break
		}
	}
	if focused == s.webFocus {
		return
	}
	old := s.webFocus
	s.webFocus = focused
	if old != nil {
		old.dispatch(driver.EventBlur)
	}
	if focused != nil {
		focused.dispatch(driver.EventFocus)
	}
}

// modalCame remembers what had the focus as modal v came: a field's
// NSTextField rather than the shared field editor it edits in.
func (s *surface) modalCame(v *view) {
	first := s.window.Send(sel("firstResponder"))
	if objc.Send[bool](first, sel("isKindOfClass:"), apple.Class("NSTextView")) &&
		objc.Send[bool](first, sel("isFieldEditor")) {
		first = first.Send(sel("delegate"))
	}
	if first != 0 && first != s.window {
		v.focusBefore = first.Send(sel("retain"))
	}
	s.modals = append(s.modals, v)
}

// modalGone gives the focus back to what had it as modal v came, if it
// fell to the window or the surface meanwhile and that can still take it.
func (s *surface) modalGone(v *view) {
	i := slices.Index(s.modals, v)
	if i < 0 {
		return
	}
	s.modals = slices.Delete(s.modals, i, i+1)
	back := v.focusBefore
	v.focusBefore = 0
	if back == 0 {
		return
	}
	defer back.Send(selRelease)
	// A newer modal that remembered a view of v remembers v's instead.
	for _, m := range s.modals[i:] {
		if m.focusBefore != 0 && objc.Send[bool](m.focusBefore, sel("isKindOfClass:"), apple.Class("NSView")) &&
			objc.Send[bool](m.focusBefore, sel("isDescendantOf:"), v.native) {
			m.focusBefore.Send(selRelease)
			m.focusBefore = back.Send(sel("retain"))
			return
		}
	}
	first := s.window.Send(sel("firstResponder"))
	if first != 0 && first != s.window && first != s.native {
		return
	}
	if back.Send(selWindow) != s.window || !objc.Send[bool](back, sel("acceptsFirstResponder")) {
		return
	}
	s.window.Send(sel("makeFirstResponder:"), back)
	// A field comes back with the caret at the end, its text unselected.
	if !objc.Send[bool](back, sel("respondsToSelector:"), sel("currentEditor")) {
		return
	}
	if editor := back.Send(sel("currentEditor")); editor != 0 {
		n := objc.Send[uint](editor.Send(sel("string")), sel("length"))
		editor.Send(sel("setSelectedRange:"), apple.NSRange{Location: n})
	}
}
