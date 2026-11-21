//go:build js && wasm

package web

import (
	"fmt"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

type jsScreen struct {
	s *surface
}

func (s *surface) isFullScreen() bool {
	return s.doc.Get("fullscreenElement").Truthy() || s.doc.Get("webkitFullscreenElement").Truthy()
}

func (s *surface) setFullScreen(on bool) {
	s.screen.setFullScreen(on)
}

func (j jsScreen) isFullScreen() bool {
	return j.s.isFullScreen()
}

func (j jsScreen) canFullScreen() bool {
	el := j.s.doc.Get("documentElement")
	if !el.Truthy() {
		return false
	}
	if el.Get("requestFullscreen").Type() != js.TypeFunction && el.Get("webkitRequestFullscreen").Type() != js.TypeFunction {
		return false
	}
	if fe := j.s.doc.Get("fullscreenEnabled"); fe.Type() == js.TypeBoolean {
		return fe.Bool()
	}
	if wfe := j.s.doc.Get("webkitFullscreenEnabled"); wfe.Type() == js.TypeBoolean {
		return wfe.Bool()
	}
	return true
}

func (j jsScreen) requestFullScreen(settle func(*screenError)) (bool, *screenError) {
	el := j.s.doc.Get("documentElement")
	if !el.Truthy() {
		return false, &screenError{Name: "NotSupportedError", Message: "full screen is not supported by this browser"}
	}
	return invokeScreenMethod(el, "requestFullscreen", "webkitRequestFullscreen", "full screen is not supported by this browser", settle)
}

func (j jsScreen) exitFullScreen(settle func(*screenError)) (bool, *screenError) {
	return invokeScreenMethod(j.s.doc, "exitFullscreen", "webkitExitFullscreen", "full screen exit is not supported by this browser", settle)
}

func (j jsScreen) orientationObject() js.Value {
	scr := js.Global().Get("screen")
	if !scr.Truthy() {
		return js.Undefined()
	}
	return scr.Get("orientation")
}

func (j jsScreen) canLockOrientation() bool {
	orient := j.orientationObject()
	return orient.Truthy() && orient.Get("lock").Type() == js.TypeFunction
}

func (j jsScreen) currentOrientation() driver.Orientation {
	if orient := j.orientationObject(); orient.Truthy() {
		if typ := orient.Get("type"); typ.Type() == js.TypeString {
			if o := parseScreenOrientation(typ.String()); o != 0 {
				return o
			}
		}
	}
	if angle := js.Global().Get("orientation"); angle.Type() == js.TypeNumber {
		return parseLegacyOrientation(angle.Int())
	}
	return 0
}

func (j jsScreen) lockOrientation(lockType string, settle func(*screenError)) (bool, *screenError) {
	orient := j.orientationObject()
	if !orient.Truthy() || orient.Get("lock").Type() != js.TypeFunction {
		return false, &screenError{Name: "NotSupportedError", Message: "screen orientation lock is not supported by this browser"}
	}
	res, err := callSafe(orient, "lock", lockType)
	if err != nil {
		return false, err
	}
	if isPromise(res) {
		then(res, func(js.Value) { settle(nil) }, func(e js.Value) { settle(jsScreenError(e)) })
		return true, nil
	}
	return false, nil
}

func (j jsScreen) unlockOrientation() *screenError {
	orient := j.orientationObject()
	if !orient.Truthy() || orient.Get("unlock").Type() != js.TypeFunction {
		return nil
	}
	res, err := callSafe(orient, "unlock")
	if err != nil {
		return err
	}
	if isPromise(res) {
		then(res, func(js.Value) {}, func(js.Value) {})
	}
	return nil
}

func (j jsScreen) updateEnv() {
	j.s.updateEnv()
}

func (j jsScreen) dispatch(ev driver.Event) {
	j.s.engine.Dispatch(ev)
}

func invokeScreenMethod(target js.Value, primary, fallback, unsupported string, settle func(*screenError)) (bool, *screenError) {
	method := ""
	switch {
	case target.Get(primary).Type() == js.TypeFunction:
		method = primary
	case target.Get(fallback).Type() == js.TypeFunction:
		method = fallback
	default:
		return false, &screenError{Name: "NotSupportedError", Message: unsupported}
	}
	res, err := callSafe(target, method)
	if err != nil {
		return false, err
	}
	if isPromise(res) {
		then(res, func(js.Value) { settle(nil) }, func(e js.Value) { settle(jsScreenError(e)) })
		return true, nil
	}
	return false, nil
}

func isPromise(v js.Value) bool {
	return v.Type() == js.TypeObject && v.Get("then").Type() == js.TypeFunction
}

func callSafe(target js.Value, method string, args ...any) (res js.Value, err *screenError) {
	defer func() {
		if r := recover(); r != nil {
			if je, ok := r.(js.Error); ok {
				err = jsScreenError(je.Value)
				return
			}
			err = &screenError{Message: fmt.Sprint(r)}
		}
	}()
	return target.Call(method, args...), nil
}

func jsScreenError(e js.Value) *screenError {
	if e.Type() == js.TypeObject {
		var name, msg string
		if n := e.Get("name"); n.Type() == js.TypeString {
			name = n.String()
		}
		if m := e.Get("message"); m.Type() == js.TypeString {
			msg = m.String()
		}
		return &screenError{Name: name, Message: msg}
	}
	return &screenError{Message: e.String()}
}
