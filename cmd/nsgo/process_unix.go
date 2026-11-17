//go:build !windows

package main

import (
	"os"
	"syscall"
)

// processAlive reports whether a process runs as pid.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer p.Release()
	return p.Signal(syscall.Signal(0)) == nil
}
