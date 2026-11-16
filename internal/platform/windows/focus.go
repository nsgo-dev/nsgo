//go:build windows

package windows

import "github.com/nsgo-dev/nsgo/internal/driver"

// focusedView is the keyboard's field or ordinary view. Windows gives
// the native focus to the surface; the backend routes it within the tree.
func (w *window) focusedView() *view {
	if in := w.focusedField(); in != nil {
		return in.v
	}
	return w.controlFocus
}

func (w *window) focusableView(v *view) bool {
	if v == nil || v.deleted || !w.shown(v) || !isAncestor(w.focusScope(), v) {
		return false
	}
	if v.field != nil {
		return !v.props.Disabled && v.field.focusable()
	}
	return v.props.FocusStop(v.events)
}

// focusView handles Ref.Focus and accessibility focus for either kind
// of stop, without moving to an invalid target or through a modal.
func (w *window) focusView(v *view) {
	if !w.focusableView(v) || w.focusedView() == v {
		return
	}
	if v.field != nil {
		w.setFocus(v.field)
		return
	}
	w.setFocus(nil)
	w.restore = nil
	w.controlFocus = v
	w.report(v, driver.Event{Kind: driver.EventFocus, ID: v.id})
	w.uiaFocusChanged(v)
	w.revealView(v)
}

func (w *window) blurControl() {
	if v := w.controlFocus; v != nil {
		w.controlFocus = nil
		if !v.deleted {
			w.report(v, driver.Event{Kind: driver.EventBlur, ID: v.id})
		}
	}
	w.restoreControl = nil
}

// checkControlFocus follows stops hidden, disabled or covered by a
// modal. A modal gives back the stop it covered when it closes.
func (w *window) checkControlFocus() {
	v := w.controlFocus
	switch {
	case v != nil && (v.deleted || !w.shown(v) || !v.props.FocusStop(v.events)):
		w.blurControl()
	case v != nil && !isAncestor(w.focusScope(), v):
		w.blurControl()
		w.restoreControl = v
	case w.focusedView() == nil && w.restoreControl != nil:
		r := w.restoreControl
		if r.deleted || !w.shown(r) || !r.props.FocusStop(r.events) {
			w.restoreControl = nil
		} else if w.focusableView(r) {
			w.focusView(r)
		}
	}
}

// controlKey activates only a focused pressable stop, once per physical
// press. A focusable view without press handlers keeps its key bindings.
func (w *window) controlKey(vk, lParam uintptr) bool {
	v := w.controlFocus
	if !w.focusableView(v) || !v.events.Has(driver.EventPress) && !v.events.Has(driver.EventPressDown) ||
		(vk != vkReturn && vk != vkSpace) || keyDown(vkControl) || keyDown(vkShift) || keyDown(vkMenu) || keyDown(vkLWin) || keyDown(vkRWin) {
		return false
	}
	if !repeated(lParam) {
		for _, kind := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
			w.report(v, driver.Event{Kind: kind, ID: v.id, Index: -1, Count: 1})
		}
	}
	return true
}
