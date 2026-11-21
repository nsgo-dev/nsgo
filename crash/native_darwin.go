//go:build darwin

package crash

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const foundationPath = "/System/Library/Frameworks/Foundation.framework/Foundation"

var (
	darwinOnce       sync.Once
	darwinNativePath atomic.Pointer[string]
	darwinPrevFn     atomic.Uintptr
	darwinSelfCb     atomic.Uintptr

	selName             objc.SEL
	selReason           objc.SEL
	selCallStackSymbols objc.SEL
	selCount            objc.SEL
	selObjectAtIndex    objc.SEL
	selUTF8String       objc.SEL
)

// installNativeHandler registers an Objective-C uncaught exception handler via
// Foundation's NSSetUncaughtExceptionHandler (macOS and iOS) using purego with
// zero C code, chaining any previously registered handler.
func installNativeHandler(nativeCrashPath string) {
	p := nativeCrashPath
	darwinNativePath.Store(&p)
	darwinOnce.Do(func() {
		lib, err := purego.Dlopen(foundationPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		var setHandler func(uintptr)
		var getHandler func() uintptr
		purego.RegisterLibFunc(&setHandler, lib, "NSSetUncaughtExceptionHandler")
		purego.RegisterLibFunc(&getHandler, lib, "NSGetUncaughtExceptionHandler")

		selName = objc.RegisterName("name")
		selReason = objc.RegisterName("reason")
		selCallStackSymbols = objc.RegisterName("callStackSymbols")
		selCount = objc.RegisterName("count")
		selObjectAtIndex = objc.RegisterName("objectAtIndex:")
		selUTF8String = objc.RegisterName("UTF8String")

		cb := purego.NewCallback(handleDarwinException)
		darwinSelfCb.Store(cb)
		if prev := getHandler(); prev != 0 && prev != cb {
			darwinPrevFn.Store(prev)
		}
		setHandler(cb)
	})
}

func handleDarwinException(excPtr uintptr) {
	defer func() { _ = recover() }()
	writeDarwinException(objc.ID(excPtr))
	if prev := darwinPrevFn.Load(); prev != 0 && prev != darwinSelfCb.Load() {
		purego.SyscallN(prev, excPtr)
	}
}

func writeDarwinException(exc objc.ID) {
	pathPtr := darwinNativePath.Load()
	if pathPtr == nil || *pathPtr == "" || exc == 0 {
		return
	}
	trace := formatDarwinException(exc)
	if trace == "" {
		return
	}
	f, err := os.OpenFile(*pathPtr, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(trace)
	_ = f.Sync()
	_ = f.Close()
}

func formatDarwinException(exc objc.ID) string {
	if exc == 0 {
		return ""
	}
	name := nsString(exc.Send(selName))
	reason := nsString(exc.Send(selReason))

	var b strings.Builder
	fmt.Fprintf(&b, "NSException: %s\n", name)
	if reason != "" {
		fmt.Fprintf(&b, "Reason: %s\n", reason)
	}
	if symbols := exc.Send(selCallStackSymbols); symbols != 0 {
		count := int(symbols.Send(selCount))
		if count > 0 {
			b.WriteString("CallStackSymbols:\n")
			for i := 0; i < count; i++ {
				frame := nsString(symbols.Send(selObjectAtIndex, uintptr(i)))
				if frame != "" {
					b.WriteString(frame)
					b.WriteByte('\n')
				}
			}
		}
	}
	return b.String()
}

func nsString(id objc.ID) string {
	if id == 0 {
		return ""
	}
	p := objc.Send[*byte](id, selUTF8String)
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
