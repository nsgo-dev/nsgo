//go:build darwin || linux

package pty

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// closeProcess starts a real child and waits until it has installed its
// signal handling, so a hang-up cannot race the shell's startup.
func closeProcess(t *testing.T, script string) *PTY {
	t.Helper()
	p, err := Start(Options{Path: "/bin/sh", Args: []string{"sh", "-c", script}, Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	cleanupProcess(t, p)
	readyProcess(t, p)
	return p
}

func cleanupProcess(t *testing.T, p *PTY) {
	t.Helper()
	t.Cleanup(func() {
		_ = p.Kill()
		_ = p.Close()
		select {
		case <-p.Exited():
		case <-time.After(5 * time.Second):
			t.Error("child did not exit during cleanup")
		}
	})
}

func readyProcess(t *testing.T, p *PTY) {
	t.Helper()
	ready := make(chan error, 1)
	go func() {
		r := bufio.NewReader(p)
		for {
			line, err := r.ReadString('\n')
			if strings.TrimSpace(line) == "ready" || err != nil {
				ready <- err
				return
			}
		}
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("child readiness: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not become ready")
	}
}

func closedProcess(t *testing.T, p *PTY, want int) {
	t.Helper()
	select {
	case <-p.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("the process outlived its terminal")
	}
	if code, err := p.Wait(); code != want || err != nil {
		t.Fatalf("exit = %d, %v; want %d", code, err, want)
	}
}

func TestCloseInheritedHangupIgnored(t *testing.T) {
	if os.Getenv("NSGO_TEST_INHERITED_HUP") != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		// Unlike signal.Ignore in this test, this makes even the Go test
		// process start with an inherited SIG_IGN, as under nohup.
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", `trap '' HUP; exec "$@"`, "sh",
			os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_INHERITED_HUP=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("inherited SIGHUP: %v\n%s", err, out)
		}
		return
	}
	if !signal.Ignored(syscall.SIGHUP) {
		t.Fatal("helper did not inherit an ignored SIGHUP")
	}
	p := closeProcess(t, "printf 'ready\\n'; exec sleep 30")
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	closedProcess(t, p, 128+int(syscall.SIGTERM))
	if !signal.Ignored(syscall.SIGHUP) {
		t.Fatal("starting a PTY changed the parent's signal handling")
	}
}

func TestCloseEscalatesIgnoringChild(t *testing.T) {
	p := closeProcess(t, "trap '' HUP TERM; printf 'ready\\n'; exec sleep 30")
	var calls sync.WaitGroup
	for range 16 {
		calls.Go(func() {
			if err := p.Close(); err != nil {
				t.Errorf("concurrent close: %v", err)
			}
		})
	}
	calls.Wait()
	closedProcess(t, p, 128+int(syscall.SIGKILL))
	if err := p.Kill(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("kill after reaping = %v, want ErrProcessDone", err)
	}
}

func TestWaitDoesNotReapStoppedChild(t *testing.T) {
	p := closeProcess(t, "printf 'ready\\n'; exec sleep 30")
	if err := p.cmd.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Exited():
		t.Fatal("a stopped child was reported as exited")
	case <-time.After(50 * time.Millisecond):
	}
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	closedProcess(t, p, 128+int(syscall.SIGKILL))
}

func TestCloseEndsGroupAfterLeaderExits(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	go func() { <-ctx.Done(); listener.Close() }()
	p, err := Start(Options{Path: os.Args[0], Args: []string{os.Args[0], "-test.run=^TestPTYGroupProcess$"},
		// The race runtime otherwise sleeps for a second in os.Exit,
		// which would keep the leader alive beyond the SIGTERM deadline.
		Env: append(os.Environ(), "NSGO_TEST_PTY_GROUP=leader", "NSGO_TEST_PTY_ADDRESS="+listener.Addr().String(),
			"GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")})
	if err != nil {
		t.Fatal(err)
	}
	cleanupProcess(t, p)
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(conn)
	if line, err := r.ReadString('\n'); line != "ready\n" || err != nil {
		t.Fatalf("group member readiness: %q, %v", line, err)
	}
	readyProcess(t, p)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	// The leader exits on HUP; its child ignores HUP and TERM. EOF on
	// this independent connection proves the child was killed too, even
	// where the container's init leaves orphaned zombies unreaped.
	if _, err := r.ReadByte(); !errors.Is(err, io.EOF) {
		t.Fatalf("group member survived close: %v", err)
	}
	closedProcess(t, p, 0)
}

func TestPTYGroupProcess(t *testing.T) {
	switch os.Getenv("NSGO_TEST_PTY_GROUP") {
	case "leader":
		hup := make(chan os.Signal, 1)
		signal.Notify(hup, syscall.SIGHUP)
		defer signal.Stop(hup)
		cmd := exec.Command(os.Args[0], "-test.run=^TestPTYGroupProcess$")
		cmd.Env = append(os.Environ(), "NSGO_TEST_PTY_GROUP=member")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer cmd.Process.Kill()
		fmt.Println("ready")
		<-hup
		os.Exit(0)
	case "member":
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM)
		conn, err := net.Dial("tcp", os.Getenv("NSGO_TEST_PTY_ADDRESS"))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		fmt.Fprintln(conn, "ready")
		io.Copy(io.Discard, conn)
	default:
		t.Skip("child-process helper")
	}
}
