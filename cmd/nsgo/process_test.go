package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestProcessAliveHelper(t *testing.T) {
	if os.Getenv("NSGO_PROCESS_HELPER") != "1" {
		return
	}
	fmt.Println("ready")
	io.Copy(io.Discard, os.Stdin)
	os.Exit(259)
}

// idleProcess runs this test executable until stop closes its input. Waiting for its greeting proves it started, without depending on a shell or sleep.
func idleProcess(t *testing.T) (pid int, stop func()) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestProcessAliveHelper$")
	cmd.Env = append(os.Environ(), "NSGO_PROCESS_HELPER=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	stop = func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		input.Close()
		err := cmd.Wait()
		var exit *exec.ExitError
		code := 259
		if runtime.GOOS != "windows" {
			code &= 255
		}
		if !errors.As(err, &exit) || exit.ExitCode() != code {
			t.Errorf("helper exit: %v, want code %d", err, code)
		}
	}
	t.Cleanup(stop)
	if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "ready\n" && line != "ready\r\n" {
		t.Fatalf("helper greeting: %q, %v", line, err)
	}
	return cmd.Process.Pid, stop
}

func TestProcessAlive(t *testing.T) {
	for _, pid := range []int{-1, 0} {
		if processAlive(pid) {
			t.Errorf("invalid pid %d is alive", pid)
		}
	}
	if !processAlive(os.Getpid()) {
		t.Fatal("this process is not alive")
	}
	// The system's first process, which a test running as an ordinary user may not signal or open: init or launchd on Unix, System on Windows.
	system := 1
	if runtime.GOOS == "windows" {
		system = 4
	}
	if !processAlive(system) {
		t.Errorf("the system process %d is not alive", system)
	}
	pid, stop := idleProcess(t)
	if !processAlive(pid) {
		t.Fatal("running child is not alive")
	}
	stop()
	if processAlive(pid) {
		t.Fatal("finished child is alive")
	}
}
