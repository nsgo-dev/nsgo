//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

// processAlive reports whether a process runs as pid. One the caller may not signal, another user's, runs too: sessionLive's token check then decides whether it is the session's.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer p.Release()
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
