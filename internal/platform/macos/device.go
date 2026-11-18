//go:build darwin && !ios

package macos

import (
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

const systemConfigurationPath = "/System/Library/Frameworks/SystemConfiguration.framework/SystemConfiguration"

// copyComputerName is SCDynamicStoreCopyComputerName, nil when SystemConfiguration does not load.
var copyComputerName = sync.OnceValue(func() func(store uintptr, encoding *uint32) uintptr {
	lib, err := purego.Dlopen(systemConfigurationPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil
	}
	var fn func(store uintptr, encoding *uint32) uintptr
	purego.RegisterLibFunc(&fn, lib, "SCDynamicStoreCopyComputerName")
	return fn
})

// hardwareModel is the hardware's model identifier, such as "Mac15,7", which never changes while the app runs.
var hardwareModel = sync.OnceValue(func() string {
	model, _ := syscall.Sysctl("hw.model")
	return model
})

// deviceIdentity returns the computer name of System Settings' Sharing pane and the hardware's model identifier. The name comes from configd rather than NSHost, whose currentHost resolves the machine's addresses and can block the main thread for seconds on a slow network; it is read on every environment refresh, so a rename reaches the app.
func deviceIdentity() (name, model string) {
	if fn := copyComputerName(); fn != nil {
		if ref := fn(0, nil); ref != 0 {
			name = apple.GoString(objc.ID(ref))
			apple.Release(ref)
		}
	}
	return name, hardwareModel()
}
