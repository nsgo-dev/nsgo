package pty

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

func openMaster() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("pty: %w", err)
	}
	var unlock int32
	if err := ioctl(master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		master.Close()
		return nil, "", fmt.Errorf("pty: unlocking: %w", err)
	}
	var n uint32
	if err := ioctl(master, syscall.TIOCGPTN, unsafe.Pointer(&n)); err != nil {
		master.Close()
		return nil, "", fmt.Errorf("pty: %w", err)
	}
	return master, "/dev/pts/" + strconv.Itoa(int(n)), nil
}

// waitForExit leaves the exited child waitable, reserving its PID until
// the process group's final signal and exec.Cmd.Wait are serialized.
func waitForExit(pid int) error {
	var info [16]uint64 // Linux's 128-byte siginfo_t; its fields are not needed.
	for {
		_, _, err := syscall.Syscall6(syscall.SYS_WAITID, 1 /* P_PID */, uintptr(pid),
			uintptr(unsafe.Pointer(&info[0])), syscall.WEXITED|syscall.WNOWAIT, 0, 0)
		if err == syscall.EINTR {
			continue
		}
		if err != 0 {
			return os.NewSyscallError("waitid", err)
		}
		return nil
	}
}
