//go:build ios

package ios

import (
	"syscall"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func deviceIdentity() (name, model string) {
	device := objc.ID(apple.Class("UIDevice")).Send(sel("currentDevice"))
	if device != 0 {
		name = apple.GoString(device.Send(sel("name")))
	}
	model, _ = syscall.Sysctl("hw.machine")
	if model == "" && device != 0 {
		model = apple.GoString(device.Send(sel("model")))
	}
	return name, model
}
