//go:build darwin && !ios

package macos

import (
	"syscall"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func deviceIdentity() (name, model string) {
	host := objc.ID(apple.Class("NSHost")).Send(sel("currentHost"))
	if host != 0 {
		name = apple.GoString(host.Send(sel("localizedName")))
	}
	model, _ = syscall.Sysctl("hw.model")
	return name, model
}
