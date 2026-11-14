//go:build darwin && !ios

package privacy

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// macOS keeps what only Macs have through the accessibility API (ApplicationServices) and CoreGraphics, whose functions are macOS 10.15's.
var macOS struct {
	once                           sync.Once
	ok                             bool
	axIsProcessTrusted             func() bool
	axIsProcessTrustedWithOptions  func(options objc.ID) bool
	cgPreflightScreenCaptureAccess func() bool
	cgRequestScreenCaptureAccess   func() bool
	cgPreflightListenEventAccess   func() bool
	cgRequestListenEventAccess     func() bool
}

func loadMacOS() bool {
	m := &macOS
	m.once.Do(func() {
		as, err1 := purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		cg, err2 := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err1 != nil || err2 != nil {
			return
		}
		for _, f := range []struct {
			fn   any
			lib  uintptr
			name string
		}{
			{&m.axIsProcessTrusted, as, "AXIsProcessTrusted"},
			{&m.axIsProcessTrustedWithOptions, as, "AXIsProcessTrustedWithOptions"},
			{&m.cgPreflightScreenCaptureAccess, cg, "CGPreflightScreenCaptureAccess"},
			{&m.cgRequestScreenCaptureAccess, cg, "CGRequestScreenCaptureAccess"},
			{&m.cgPreflightListenEventAccess, cg, "CGPreflightListenEventAccess"},
			{&m.cgRequestListenEventAccess, cg, "CGRequestListenEventAccess"},
		} {
			p, err := purego.Dlsym(f.lib, f.name)
			if err != nil {
				return
			}
			purego.RegisterFunc(f.fn, p)
		}
		m.ok = true
	})
	return m.ok
}

// platformStatus answers what only Macs keep, and the media library, which macOS never tells.
func platformStatus(name string) int {
	switch name {
	case driver.PermissionAccessibility, driver.PermissionScreenRecording, driver.PermissionInputMonitoring:
		if !loadMacOS() {
			return driver.PermissionUnknown
		}
		switch name {
		case driver.PermissionAccessibility:
			return granted(macOS.axIsProcessTrusted())
		case driver.PermissionScreenRecording:
			return granted(macOS.cgPreflightScreenCaptureAccess())
		}
		return granted(macOS.cgPreflightListenEventAccess())
	case driver.PermissionFullDiskAccess:
		return fullDiskAccess()
	case driver.PermissionAppManagement, driver.PermissionDeveloperTools, driver.PermissionMediaLibrary:
		return driver.PermissionUnknown
	}
	return driver.PermissionUnavailable
}

// platformAsks says Request answers name with platformRequest: what only Macs keep, whose prompts need no usage description, and the media library, which macOS never asks.
func platformAsks(name string) bool {
	switch name {
	case driver.PermissionMicrophone, driver.PermissionCamera, driver.PermissionCalendars, driver.PermissionReminders, driver.PermissionBluetooth:
		return false
	}
	return true
}

// platformRequest asks for what only Macs keep: Accessibility, Screen Recording and Input Monitoring list the app in their panes as they ask, still off, and answer at once.
func platformRequest(name string) int {
	st := platformStatus(name)
	if st == driver.PermissionGranted || !loadMacOS() {
		return st
	}
	switch name {
	case driver.PermissionAccessibility:
		var trusted bool
		withString("AXTrustedCheckOptionPrompt", func(key objc.ID) {
			options := class("NSDictionary").Send(sel("dictionaryWithObject:forKey:"), class("NSNumber").Send(sel("numberWithBool:"), true), key)
			trusted = macOS.axIsProcessTrustedWithOptions(options)
		})
		return granted(trusted)
	case driver.PermissionScreenRecording:
		return granted(macOS.cgRequestScreenCaptureAccess())
	case driver.PermissionInputMonitoring:
		return granted(macOS.cgRequestListenEventAccess())
	}
	return st
}

// platformAskOnce is never reached on macOS, whose own permissions platformAsks takes.
func platformAskOnce(name string, answer func(int)) { answer(Status(name)) }

// fullDiskAccess reads what only apps with Full Disk Access read: TCC's database of the user, then Safari's files.
func fullDiskAccess() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return driver.PermissionUnknown
	}
	f, err := os.Open(filepath.Join(home, "Library/Application Support/com.apple.TCC/TCC.db"))
	if err == nil {
		f.Close()
		return driver.PermissionGranted
	}
	if errors.Is(err, fs.ErrPermission) {
		return driver.PermissionDenied
	}
	if _, err := os.ReadDir(filepath.Join(home, "Library/Safari")); err == nil {
		return driver.PermissionGranted
	} else if errors.Is(err, fs.ErrPermission) {
		return driver.PermissionDenied
	}
	return driver.PermissionUnknown
}

// ResponsibleExecutable is the executable of the process macOS asks on behalf of, which an app started from another app, as a terminal starts nsgo dev's, is not; ok is false where the system cannot name it.
func ResponsibleExecutable() (path string, ok bool) {
	responsible, err := purego.Dlsym(purego.RTLD_DEFAULT, "responsibility_get_pid_responsible_for_pid")
	if err != nil {
		return "", false
	}
	pidPath, err := purego.Dlsym(purego.RTLD_DEFAULT, "proc_pidpath")
	if err != nil {
		return "", false
	}
	pid, _, _ := purego.SyscallN(responsible, uintptr(os.Getpid()))
	buf := make([]byte, 4096)
	n, _, _ := purego.SyscallN(pidPath, pid, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if int32(n) <= 0 {
		return "", false
	}
	return string(buf[:n]), true
}

// DeclaresUsage reports whether the app macOS asks on behalf of declares key in its Info.plist: the responsible process, which the system ends this one for when it declares none. Where the system cannot name that process, it says yes.
func DeclaresUsage(key string) bool {
	executable, ok := ResponsibleExecutable()
	return !ok || bundleDeclares(executable, key)
}

// bundleDeclares says the Info.plist of the app bundle holding executable declares key; a program outside a bundle declares none. Keys are plain ASCII in both XML and binary property lists.
func bundleDeclares(executable, key string) bool {
	end := strings.LastIndex(executable, ".app/Contents/")
	if end < 0 {
		return false
	}
	info, err := os.ReadFile(executable[:end] + ".app/Contents/Info.plist")
	return err == nil && strings.Contains(string(info), key)
}
