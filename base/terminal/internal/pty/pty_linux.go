package pty

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
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

// waitForExit leaves the exited child waitable, reserving its PID until the process group's final signal and exec.Cmd.Wait are serialized.
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

// sessionGroups returns the process groups of the processes in session sid, read from /proc; nil if /proc cannot be read.
func sessionGroups(sid int) []int {
	names, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	want := strconv.Itoa(sid)
	var groups []int
	for _, e := range names {
		if e.Name()[0] < '0' || e.Name()[0] > '9' {
			continue
		}
		stat, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue // The process exited, or is not ours to see.
		}
		// pid (comm) state ppid pgrp session ...; comm may hold spaces and parentheses.
		i := bytes.LastIndexByte(stat, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(string(stat[i+1:]))
		if len(f) < 4 || f[3] != want {
			continue
		}
		if g, err := strconv.Atoi(f[2]); err == nil && !slices.Contains(groups, g) {
			groups = append(groups, g)
		}
	}
	return groups
}
