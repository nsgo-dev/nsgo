//go:build js && wasm

package web

import "syscall/js"

type fullScreenTransition struct {
	pending bool
	on      bool
	reverse bool
}

func (s *surface) isFullScreen() bool {
	return s.doc.Get("fullscreenElement").Truthy() || s.doc.Get("webkitFullscreenElement").Truthy()
}

func (s *surface) setFullScreen(on bool) {
	f := &s.fullScreenTransition
	if f.pending {
		f.reverse = on != f.on
		return
	}
	if s.isFullScreen() == on {
		return
	}
	target, method, fallback := s.doc, "exitFullscreen", "webkitExitFullscreen"
	if on {
		target, method, fallback = s.doc.Get("documentElement"), "requestFullscreen", "webkitRequestFullscreen"
	}
	if target.Get(method).Type() != js.TypeFunction {
		call(target, fallback)
		return
	}
	promise := target.Call(method)
	if promise.Type() != js.TypeObject || promise.Get("then").Type() != js.TypeFunction {
		return
	}
	f.pending, f.on = true, on
	settled := func(js.Value) {
		reverse := f.reverse
		*f = fullScreenTransition{}
		if reverse {
			s.setFullScreen(!on)
		}
	}
	then(promise, settled, settled)
}
