//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A page has of the system's permissions the microphone and the camera: the Permissions API tells them, and getUserMedia asks for them. Firefox, whose Permissions API names neither, tells nothing until asked, and a page outside a secure context has neither. A page opens no settings and guides nowhere.

// permission carries out an ItemPermission numbered id.
func (s *surface) permission(id uint32, r driver.PermissionRequest) {
	dispatch := s.engine.Dispatch
	answer := func(status int) { dispatch(driver.Event{Kind: driver.EventPermission, Count: id, Index: status}) }
	if r.Name != driver.PermissionMicrophone && r.Name != driver.PermissionCamera || r.Command == driver.PermissionStopGuide || r.Command == driver.PermissionOpenSettings {
		driver.AnswerPermission(id, r, func(string) int { return driver.PermissionUnavailable }, dispatch)
		return
	}
	if r.Command != driver.PermissionAsk {
		mediaPermission(r.Name, answer)
		return
	}
	mediaPermission(r.Name, func(status int) {
		if status != driver.PermissionNotAsked && status != driver.PermissionUnknown {
			answer(status)
			return
		}
		askMedia(r.Name, answer)
	})
}

// mediaPermission answers the status of the microphone or the camera, by the Permissions API's name for it.
func mediaPermission(name string, answer func(int)) {
	nav := js.Global().Get("navigator")
	if !nav.Get("mediaDevices").Truthy() {
		answer(driver.PermissionUnavailable)
		return
	}
	permissions := nav.Get("permissions")
	if !permissions.Truthy() {
		answer(driver.PermissionUnknown)
		return
	}
	query, ok := tryCall(permissions, "query", map[string]any{"name": name})
	if !ok {
		answer(driver.PermissionUnknown)
		return
	}
	then(query, func(status js.Value) { answer(permissionState(status.Get("state").String())) }, func(js.Value) { answer(driver.PermissionUnknown) })
}

// permissionState is the status of a PermissionStatus's state.
func permissionState(state string) int {
	switch state {
	case "granted":
		return driver.PermissionGranted
	case "denied":
		return driver.PermissionDenied
	case "prompt":
		return driver.PermissionNotAsked
	}
	return driver.PermissionUnknown
}

// askMedia has the browser ask for the microphone or the camera by opening it, closing what it opened at once.
func askMedia(name string, answer func(int)) {
	constraints := map[string]any{"audio": true}
	if name == driver.PermissionCamera {
		constraints = map[string]any{"video": true}
	}
	stream, ok := tryCall(js.Global().Get("navigator").Get("mediaDevices"), "getUserMedia", constraints)
	if !ok {
		answer(driver.PermissionUnknown)
		return
	}
	then(stream, func(stream js.Value) {
		tracks := stream.Call("getTracks")
		for i := range tracks.Length() {
			tracks.Index(i).Call("stop")
		}
		answer(driver.PermissionGranted)
	}, func(err js.Value) {
		// Chrome rejects a dismissed prompt as it does a blocked one, while the Permissions API still reads "prompt" for it: only a browser whose API cannot tell takes the rejection for a refusal.
		refused := err.Truthy() && err.Get("name").String() == "NotAllowedError"
		mediaPermission(name, func(status int) {
			if refused && status == driver.PermissionUnknown {
				status = driver.PermissionDenied
			}
			answer(status)
		})
	})
}

// tryCall calls method of v with args, reporting false where it throws, as an insecure context's getUserMedia may.
func tryCall(v js.Value, method string, args ...any) (result js.Value, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return v.Call(method, args...), true
}
