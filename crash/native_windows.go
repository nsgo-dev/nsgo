//go:build windows

package crash

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"structs"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const exceptionContinueSearch uintptr = 0

// exceptionRecord matches Win32 EXCEPTION_RECORD on 64-bit Windows.
type exceptionRecord struct {
	_                    structs.HostLayout
	ExceptionCode        uint32
	ExceptionFlags       uint32
	ExceptionRecord      *exceptionRecord
	ExceptionAddress     uintptr
	NumberParameters     uint32
	ExceptionInformation [15]uintptr
}

// exceptionPointers matches Win32 EXCEPTION_POINTERS.
type exceptionPointers struct {
	_               structs.HostLayout
	ExceptionRecord *exceptionRecord
	ContextRecord   uintptr
}

var (
	winOnce       sync.Once
	winNativePath atomic.Pointer[string]
	winPrevFilter atomic.Uintptr
	winSelfCb     atomic.Uintptr
)

// installNativeHandler registers a Win32 unhandled exception filter via
// kernel32.dll's SetUnhandledExceptionFilter using syscall with zero C code.
func installNativeHandler(nativeCrashPath string) {
	p := nativeCrashPath
	winNativePath.Store(&p)
	winOnce.Do(func() {
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		proc := kernel32.NewProc("SetUnhandledExceptionFilter")
		if err := proc.Find(); err != nil {
			return
		}
		cb := syscall.NewCallback(winUnhandledExceptionFilter)
		winSelfCb.Store(cb)
		prev, _, _ := proc.Call(cb)
		if prev != 0 && prev != cb {
			winPrevFilter.Store(prev)
		}
	})
}

func winUnhandledExceptionFilter(ep *exceptionPointers) uintptr {
	defer func() { _ = recover() }()
	if pathPtr := winNativePath.Load(); pathPtr != nil && *pathPtr != "" {
		trace := formatWindowsException(ep)
		if f, err := os.OpenFile(*pathPtr, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
			_, _ = f.WriteString(trace)
			_ = f.Sync()
			_ = f.Close()
		}
	}
	if prev := winPrevFilter.Load(); prev != 0 && prev != winSelfCb.Load() {
		ret, _, _ := syscall.SyscallN(prev, uintptr(unsafe.Pointer(ep)))
		return ret
	}
	return exceptionContinueSearch
}

func formatWindowsException(ep *exceptionPointers) string {
	var b strings.Builder
	if ep == nil || ep.ExceptionRecord == nil {
		b.WriteString("Windows Unhandled Exception: unknown (nil EXCEPTION_POINTERS)\n")
	} else {
		rec := ep.ExceptionRecord
		name := windowsExceptionName(rec.ExceptionCode)
		if name != "" {
			fmt.Fprintf(&b, "Windows Unhandled Exception: %s (0x%08X)\n", name, rec.ExceptionCode)
		} else {
			fmt.Fprintf(&b, "Windows Unhandled Exception: 0x%08X\n", rec.ExceptionCode)
		}
		fmt.Fprintf(&b, "Address: 0x%X\n", rec.ExceptionAddress)
		fmt.Fprintf(&b, "Flags: 0x%08X\n", rec.ExceptionFlags)
		n := int(rec.NumberParameters)
		if n > len(rec.ExceptionInformation) {
			n = len(rec.ExceptionInformation)
		}
		if n > 0 {
			b.WriteString("Parameters:")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&b, " 0x%X", rec.ExceptionInformation[i])
			}
			b.WriteByte('\n')
		}
	}
	if stack := debug.Stack(); len(stack) > 0 {
		b.WriteString("\nGo Stack:\n")
		b.Write(stack)
	}
	return b.String()
}

func windowsExceptionName(code uint32) string {
	switch code {
	case 0xC0000005:
		return "EXCEPTION_ACCESS_VIOLATION"
	case 0xC0000006:
		return "EXCEPTION_IN_PAGE_ERROR"
	case 0xC000001D:
		return "EXCEPTION_ILLEGAL_INSTRUCTION"
	case 0xC0000025:
		return "EXCEPTION_NONCONTINUABLE_EXCEPTION"
	case 0xC000008C:
		return "EXCEPTION_ARRAY_BOUNDS_EXCEEDED"
	case 0xC000008D:
		return "EXCEPTION_FLT_DENORMAL_OPERAND"
	case 0xC000008E:
		return "EXCEPTION_FLT_DIVIDE_BY_ZERO"
	case 0xC0000094:
		return "EXCEPTION_INT_DIVIDE_BY_ZERO"
	case 0xC0000095:
		return "EXCEPTION_INT_OVERFLOW"
	case 0xC0000096:
		return "EXCEPTION_PRIV_INSTRUCTION"
	case 0xC00000FD:
		return "EXCEPTION_STACK_OVERFLOW"
	case 0x80000003:
		return "EXCEPTION_BREAKPOINT"
	default:
		return ""
	}
}
