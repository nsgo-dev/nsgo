package main

import (
	"os"
	"syscall"
	"testing"
)

func TestProcessAliveAfterExitWithRetainedHandle(t *testing.T) {
	pid, stop := idleProcess(t)
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	stop()
	// Keep the exited process object available to OpenProcess. Its exit code is 259, which GetExitCodeProcess also uses to mean STILL_ACTIVE.
	if processAlive(pid) {
		t.Fatal("an exited process with a retained handle is alive")
	}
}

func TestProcessAliveRejectsTruncatedPID(t *testing.T) {
	pid := int64(os.Getpid()) + 1<<32
	if int64(int(pid)) != pid {
		t.Skip("needs 64-bit process IDs")
	}
	if processAlive(int(pid)) {
		t.Fatal("an invalid PID was truncated to this process's PID")
	}
}
