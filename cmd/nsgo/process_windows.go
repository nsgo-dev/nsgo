package main

import "syscall"

// processAlive reports whether a process runs as pid.
func processAlive(pid int) bool {
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)) {
		return false
	}
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	// An exited process is signaled, even if its exit code is STILL_ACTIVE (259). A zero timeout checks its state without waiting for it to exit.
	state, err := syscall.WaitForSingleObject(h, 0)
	return err == nil && state == syscall.WAIT_TIMEOUT
}
