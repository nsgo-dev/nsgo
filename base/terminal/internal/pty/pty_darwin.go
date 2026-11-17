package pty

import (
	"bytes"
	"fmt"
	"os"
	"slices"
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

// waitForExit observes exit without reaping, as Linux's WNOWAIT does. Darwin's waitid can also return for stopped children despite WEXITED.
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

// The sysctl listing the processes of a real user ID (sys/sysctl.h), and the layout of the kinfo_proc entries it returns on 64-bit Darwin: the size of the struct and the offset of kp_proc.p_pid. Darwin answers KERN_PROC_SESSION with ENOENT.
const (
	ctlKern            = 1
	kernProc           = 14
	kernProcRUID       = 6
	kinfoProcSize      = 648
	kinfoProcPidOffset = 40
)

// sessionGroups returns the process groups of this user's processes in session sid; nil if the kernel does not list them. Processes of other users could not be signalled anyway.
func sessionGroups(sid int) []int {
	mib := [4]int32{ctlKern, kernProc, kernProcRUID, int32(syscall.Getuid())}
	buf := userProcs(mib[:])
	var groups []int
	for off := 0; off+kinfoProcSize <= len(buf); off += kinfoProcSize {
		pid := int(*(*int32)(unsafe.Pointer(&buf[off+kinfoProcPidOffset])))
		// Only a process still in session sid is the terminal's; getsid(0) and getpgid(0) would name this process's own.
		if s, err := syscall.Getsid(pid); pid <= 0 || err != nil || s != sid {
			continue
		}
		if g, err := syscall.Getpgid(pid); err == nil && !slices.Contains(groups, g) {
			groups = append(groups, g)
		}
	}
	return groups
}

// userProcs returns the kinfo_proc entries the sysctl mib lists.
func userProcs(mib []int32) []byte {
	for range 4 {
		var n uintptr
		if sysctl(mib, nil, &n) != nil || n == 0 {
			return nil
		}
		n += n / 4 // Room for processes started since.
		buf := make([]byte, n)
		switch err := sysctl(mib, &buf[0], &n); err {
		case nil:
			return buf[:n]
		case syscall.ENOMEM:
			continue // More processes started than there was room for.
		default:
			return nil
		}
	}
	return nil
}

func sysctl(mib []int32, old *byte, n *uintptr) error {
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(old)), uintptr(unsafe.Pointer(n)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
