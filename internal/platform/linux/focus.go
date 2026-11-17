//go:build linux && !android

package linux

import (
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Focus stays in the newest modal. Ordinary widgets become focusable only when Tab or an explicit command chooses them, so GTK's automatic focus on mapping a window does not focus a button on the app's behalf.
func (w *window) focusScope() *view {
	root := w.view(w.root)
	scope := root
	var walk func(*view)
	walk = func(v *view) {
		if v.style.Hidden {
			return
		}
		if v.kind == driver.KindModal {
			scope = v
		}
		for _, c := range v.children {
			walk(c)
		}
	}
	if root != nil {
		walk(root)
	}
	return scope
}

// focusableView reports whether v is a keyboard stop: an enabled field or a stop view that shows within the newest modal.
func (w *window) focusableView(v *view) bool { return w.focusableIn(v, w.focusScope()) }

// focusableIn is focusableView for a focus scope the caller found once, as Tab does for every view.
func (w *window) focusableIn(v, scope *view) bool {
	if v == nil || v.deleted {
		return false
	}
	inside := false
	for p := v; p != nil; p = p.parent {
		if p.style.Hidden {
			return false
		}
		inside = inside || p == scope
		if p == w.view(w.root) {
			if v.kind == driver.KindInput {
				return inside && !v.props.Disabled && !v.input.Disabled
			}
			return inside && v.props.FocusStop(v.events)
		}
	}
	return false
}

func (w *window) focusedView() *view {
	if w.controlFocus != nil {
		return w.controlFocus
	}
	return w.focusedEntry()
}

// focusView grabs the actual GTK widget; focus reports then drive the engine's focus and focus-visible styles as they do for text fields.
func (w *window) focusView(v *view) {
	if !w.focusableView(v) {
		return
	}
	if v.entry != 0 {
		w.focusEntry(v)
		return
	}
	gtkWidgetSetFocusable(v.widget, 1)
	if gtkWidgetGrabFocus(v.widget) != 0 {
		w.setControlFocus(v)
	}
}

func (w *window) setControlFocus(v *view) {
	old := w.controlFocus
	if old == v {
		return
	}
	w.controlFocus = v
	if old != nil && !old.deleted {
		gtkWidgetSetFocusable(old.widget, 0)
		if old.events.Has(driver.EventBlur) {
			w.engine.Dispatch(driver.Event{Kind: driver.EventBlur, ID: old.id})
		}
	}
	if v != nil && v.events.Has(driver.EventFocus) {
		w.engine.Dispatch(driver.Event{Kind: driver.EventFocus, ID: v.id})
	}
}

func controlFocusCallback(widget, pspec, data uintptr) {
	r, ok := lookupWidget(widget)
	if !ok || r.v == nil {
		return
	}
	w, v := r.w, r.v
	if top := w.toplevel(); top != 0 && gtkWindowGetFocus(top) == widget {
		w.setControlFocus(v)
	} else if w.controlFocus == v {
		w.setControlFocus(nil)
	}
}

func (w *window) checkControlFocus() {
	v := w.controlFocus
	if v == nil {
		return
	}
	if !w.focusableView(v) {
		w.focusEntry(nil)
	}
}

func (w *window) tab(back bool) bool {
	scope := w.focusScope()
	var stops []*view
	var walk func(*view)
	walk = func(v *view) {
		if v.style.Hidden {
			return
		}
		if w.focusableIn(v, scope) {
			stops = append(stops, v)
		}
		for _, c := range v.children {
			walk(c)
		}
	}
	if scope != nil {
		walk(scope)
	}
	if len(stops) == 0 {
		return false
	}
	next := 0
	if back {
		next = len(stops) - 1
	}
	if i := slices.Index(stops, w.focusedView()); i >= 0 {
		if back {
			next = (i + len(stops) - 1) % len(stops)
		} else {
			next = (i + 1) % len(stops)
		}
	}
	w.focusView(stops[next])
	return true
}

// controlKey presses the focused pressable stop on Return or Space, once per physical press, after the keymap has had the key. A focusable view without press handlers leaves the key alone.
func (w *window) controlKey(keyval, state uint32, repeat bool) bool {
	v := w.controlFocus
	if !w.focusableView(v) || !v.events.Pressable() ||
		(keyval != gdkReturn && keyval != gdkKPEnter && keyval != gdkSpace) || state&(gdkShift|gdkControl|gdkAlt|gdkSuper|gdkMeta) != 0 {
		return false
	}
	if !repeat {
		for _, kind := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
			if v.events.Has(kind) {
				w.engine.Dispatch(driver.Event{Kind: kind, ID: v.id, Index: -1, Count: 1})
			}
		}
	}
	return true
}
