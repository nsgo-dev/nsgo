package core

import (
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

type errorHandler struct {
	owner *Scope
	fn    func(error)
}

type orientationRequest struct{ mask Orientation }

type orientationRequests struct {
	live   []*orientationRequest
	sent   Orientation
	failed bool
	dirty  bool
}

func (r *orientationRequests) current() Orientation {
	if len(r.live) == 0 {
		return 0
	}
	return r.live[len(r.live)-1].mask
}

// RequestOrientation asks iOS, Android and browsers supporting Screen Orientation
// lock to show the app in one of the given interface orientations while the
// current scope lives. The newest live request wins. When none remains, the
// system applies the app's usual orientation policy (unlocking on the web).
// A zero mask makes no request. On iOS a presented modal's SupportedOrientations
// takes precedence. On the web, call from a user gesture (with desktop.SetFullScreen
// when the browser requires full screen); browser refusals report through
// OnOrientationError. Desktop targets ignore the request.
func (cx *Scope) RequestOrientation(mask Orientation) {
	mask &= OrientationPortrait | OrientationPortraitUpsideDown | OrientationLandscape
	owner := cx.current()
	t := owner.rt.tree
	if t == nil || mask == 0 || owner.disposed {
		return
	}
	r := &t.orientations
	previous := r.current()
	request := &orientationRequest{mask}
	r.live = append(r.live, request)
	if mask != previous || r.failed {
		r.dirty = true
		t.changed = true
	}
	owner.addCleanup(func() {
		previous := r.current()
		r.live = slices.DeleteFunc(r.live, func(live *orientationRequest) bool { return live == request })
		if r.current() != previous {
			r.dirty = true
			t.changed = true
		}
	})
}

// OnOrientationError runs fn, while the current scope lives, when the platform
// refuses or fails to lock or unlock the interface orientation, such as Safari
// rejecting screen.orientation.lock (CodeUnavailable) or a browser refusing a
// lock outside full screen or a user gesture (CodeDenied).
func (cx *Scope) OnOrientationError(fn func(error)) {
	if fn == nil {
		return
	}
	owner := cx.current()
	t := owner.rt.tree
	if t == nil || owner.disposed {
		return
	}
	h := &errorHandler{owner: owner, fn: fn}
	t.orientationErrors = append(t.orientationErrors, h)
	owner.addCleanup(func() {
		t.orientationErrors = slices.DeleteFunc(t.orientationErrors, func(o *errorHandler) bool { return o == h })
	})
}

func (t *Tree) emitOrientation() {
	r := &t.orientations
	mask := r.current()
	if r.dirty || mask != r.sent {
		r.dirty = false
		r.failed = false
		r.sent = mask
		t.items = append(t.items, driver.MountItem{Kind: driver.ItemOrientation, Count: uint32(mask)})
	}
}
