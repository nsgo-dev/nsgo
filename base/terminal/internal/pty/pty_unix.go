//go:build darwin || linux

package pty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type sys struct {
	master *os.File
	cmd    *exec.Cmd
	// mu keeps group signals and reaping together: a reaped PID could name an unrelated process group or session by the next escalation.
	mu        sync.Mutex
	reaped    bool
	closing   bool
	groupSafe bool
	shutdown  chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func (p *PTY) start(o Options) error {
	master, slaveName, err := openMaster()
	if err != nil {
		return err
	}
	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return fmt.Errorf("pty: opening %s: %w", slaveName, err)
	}
	defer slave.Close() // the child has its own copies
	if err := setSize(master, o.Cols, o.Rows, o.Width, o.Height); err != nil {
		master.Close()
		return err
	}
	cmd := &exec.Cmd{Path: o.Path, Args: o.Args, Dir: o.Dir, Env: o.Env, Stdin: slave, Stdout: slave, Stderr: slave}
	// A session of its own, whose controlling terminal is the slave (the
	// child's descriptor 0), so that the terminal's signals reach the
	// process group in the foreground.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		master.Close()
		return err
	}
	p.master, p.cmd = master, cmd
	p.groupSafe, p.shutdown = true, make(chan struct{})
	return nil
}

func (p *PTY) wait() (int, error) {
	// Observe exit without reaping. Once Close starts, keep the child's PID reserved until the last group signal has been sent.
	readyErr := waitForExit(p.cmd.Process.Pid)
	p.mu.Lock()
	if readyErr != nil {
		// If the kernel cannot watch exit, os.Process still signals its child safely. Disable raw group signals before Wait may reap it.
		p.groupSafe = false
	}
	if p.closing {
		p.mu.Unlock()
		<-p.shutdown
		p.mu.Lock()
	}
	if readyErr != nil {
		// Wait may block on this fallback, so allow Close to signal it.
		p.mu.Unlock()
	}
	err := p.cmd.Wait()
	if readyErr != nil {
		p.mu.Lock()
	}
	p.reaped = true
	p.mu.Unlock()
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
		if ws, ok := e.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), readyErr
		}
		return e.ExitCode(), readyErr
	}
	return 0, errors.Join(err, readyErr)
}

// Read reads the output of the process; it returns io.EOF once no process
// has the terminal open any more, or after Close.
func (p *PTY) Read(b []byte) (int, error) {
	n, err := p.master.Read(b)
	if err != nil && n == 0 {
		// Linux reports EIO when the last process with the terminal
		// open closes it.
		if errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) {
			err = io.EOF
		}
	}
	return n, err
}

// Write types b into the terminal.
func (p *PTY) Write(b []byte) (int, error) {
	n, err := p.master.Write(b)
	if errors.Is(err, os.ErrClosed) {
		err = ErrClosed
	}
	return n, err
}

// Resize sets the size of the terminal, which sends the process SIGWINCH.
func (p *PTY) Resize(cols, rows, width, height int) error {
	return setSize(p.master, cols, rows, width, height)
}

// ClearScreen does nothing: the terminal has no screen of its own to
// clear, unlike ConPTY's console.
func (p *PTY) ClearScreen() error { return nil }

// Pid returns the process's ID.
func (p *PTY) Pid() int { return p.cmd.Process.Pid }

// Close hangs up the terminal and the process groups of its session: the process's and those of the jobs it started. Groups ignoring SIGHUP get SIGTERM after a grace period, then SIGKILL. It does not wait for exit; Wait includes the session's shutdown.
func (p *PTY) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closing = !p.reaped
		active := p.closing
		p.mu.Unlock()
		if active {
			_ = p.signal(syscall.SIGHUP)
		}
		p.closeErr = p.master.Close()
		if !active {
			return
		}
		go func() {
			defer close(p.shutdown)
			for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
				timer := time.NewTimer(closeGrace)
				<-timer.C
				_ = p.signal(sig)
			}
		}()
	})
	return p.closeErr
}

const closeGrace = 250 * time.Millisecond

// Kill kills the process and the process groups of its session, which it leads.
func (p *PTY) Kill() error {
	return p.signal(syscall.SIGKILL)
}

func (p *PTY) signal(sig syscall.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reaped {
		return os.ErrProcessDone
	}
	if p.groupSafe {
		pid := p.cmd.Process.Pid
		err := syscall.Kill(-pid, sig)
		// An interactive shell runs each job in a process group of its own, in its session. The unreaped leader keeps the session's ID, its PID, from naming anything else.
		for _, g := range sessionGroups(pid) {
			if g != pid && g > 1 {
				_ = syscall.Kill(-g, sig)
			}
		}
		if err != syscall.ESRCH {
			return err
		}
	}
	return p.cmd.Process.Signal(sig)
}

func setSize(f *os.File, cols, rows, width, height int) error {
	ws := struct{ row, col, xpixel, ypixel uint16 }{clamp(rows), clamp(cols), clamp(width), clamp(height)}
	return ioctl(f, syscall.TIOCSWINSZ, unsafe.Pointer(&ws))
}

func clamp(v int) uint16 { return uint16(max(0, min(v, 0xFFFF))) }

// ioctl calls ioctl on f with a pointer, which it converts in the system
// call, so that what it points to stays put meanwhile.
func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	c, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var errno syscall.Errno
	err = c.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	})
	if err != nil {
		return err
	}
	if errno != 0 {
		return errno
	}
	return nil
}
