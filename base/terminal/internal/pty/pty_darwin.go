package pty

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// The ioctls of grantpt, unlockpt and ptsname on macOS (sys/ttycom.h).
const (
	tiocptygrant = 0x20007454 // _IO('t', 84)
	tiocptyunlk  = 0x20007452 // _IO('t', 82)
	tiocptygname = 0x40807453 // _IOC(IOC_OUT, 't', 83, 128)
)

func openMaster() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("pty: %w", err)
	}
	var name [128]byte
	for _, step := range []struct {
		req uintptr
		arg unsafe.Pointer
	}{{tiocptygrant, nil}, {tiocptyunlk, nil}, {tiocptygname, unsafe.Pointer(&name[0])}} {
		if err := ioctl(master, step.req, step.arg); err != nil {
			master.Close()
			return nil, "", fmt.Errorf("pty: %w", err)
		}
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	return master, string(name[:n]), nil
}

// waitForExit observes exit without reaping, as Linux's WNOWAIT does.
// Darwin's waitid can also return for stopped children despite WEXITED.
func waitForExit(pid int) error {
	fd, err := syscall.Kqueue()
	if err != nil {
		return os.NewSyscallError("kqueue", err)
	}
	defer syscall.Close(fd)
	syscall.CloseOnExec(fd)
	change := syscall.Kevent_t{Ident: uint64(pid), Filter: syscall.EVFILT_PROC,
		Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT}
	for {
		_, err = syscall.Kevent(fd, []syscall.Kevent_t{change}, nil, nil)
		if err != syscall.EINTR {
			break
		}
	}
	if err == syscall.ESRCH {
		return nil // The child already exited; no one has reaped it.
	}
	if err != nil {
		return os.NewSyscallError("kevent", err)
	}
	var events [1]syscall.Kevent_t
	for {
		n, err := syscall.Kevent(fd, nil, events[:], nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return os.NewSyscallError("kevent", err)
		}
		if n > 0 {
			if events[0].Flags&syscall.EV_ERROR != 0 {
				return os.NewSyscallError("kevent", syscall.Errno(events[0].Data))
			}
			return nil
		}
	}
}
