package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// iosTestRunner installs the test build in sys's simulator (-device's, else the booted one, else the newest iPhone or Apple TV) and launches it for each test, its output streamed back. A package whose TestMain starts the native test host runs as the app, its result read between the host's marks; any other package, a library's among them, runs as a plain process through simctl spawn.
func iosTestRunner(ctx context.Context, p *project, sys *appleOS, dir string, opts testOptions) (targetRunner, error) {
	app, err := buildIOSApp(ctx, p, sys, dir, true)
	if err != nil {
		return targetRunner{}, err
	}
	udid, err := bootSimulator(ctx, sys, opts.device)
	if err != nil {
		return targetRunner{}, err
	}
	if err := run(ctx, "xcrun", "simctl", "install", udid, app); err != nil {
		return targetRunner{}, err
	}
	simulator := simulatorTest{udid: udid, id: p.ID, timeout: opts.timeout, launchTimeout: simulatorLaunchTimeout}
	target := sys.target + "_simulator"
	hosted, err := linksPlatform(ctx, p, target)
	if err != nil {
		return targetRunner{}, err
	}
	if hosted {
		// Only nsgotest.Main starts the host that writes the marks; a package that links it without a TestMain runs its tests as a plain process, as before the marks.
		funcs, err := testFuncs(ctx, p, target)
		if err != nil {
			return targetRunner{}, err
		}
		hosted = slices.Contains(funcs, "TestMain")
	}
	if !hosted {
		simulator.binary = filepath.Join(app, p.Name)
	}
	// The test host's start mark starts a hosted test's deadline, so the app's launch, which a busy machine can stretch to minutes, does not count against it. A plain process has no mark and keeps the run's deadline.
	return targetRunner{run: simulator.run, close: simulator.stop, timesFromStart: hosted}, nil
}

// simulatorLaunchTimeout bounds how long a simulator app may take to start the test host, which -timeout does not cover.
const simulatorLaunchTimeout = 5 * time.Minute

var (
	errSimulatorLaunchTimeout = errors.New("the simulator app's launch timed out")
	errSimulatorTestTimeout   = errors.New("the simulator test timed out")
)

type simulatorTest struct {
	udid, id      string
	binary        string        // the test binary inside the app, for a package that does not start the native host: it runs as a plain process through simctl spawn
	timeout       time.Duration // the test's own deadline, from the host's start mark; 0 for none
	launchTimeout time.Duration // the deadline for the host's start mark; 0 for simulatorLaunchTimeout
}

func (s simulatorTest) stop() {
	// A stalled CoreSimulator must not turn a test timeout into an unbounded wait during cleanup. The test's context may be cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exec.CommandContext(ctx, "xcrun", "simctl", "terminate", s.udid, s.id).Run()
}

func (s simulatorTest) run(ctx context.Context, args, env []string, out io.Writer) (int, error) {
	if s.binary != "" {
		cmd := exec.CommandContext(ctx, "xcrun", append([]string{"simctl", "spawn", s.udid, s.binary}, args...)...)
		cmd.Env = os.Environ()
		for _, kv := range env {
			cmd.Env = append(cmd.Env, "SIMCTL_CHILD_"+kv)
		}
		cmd.Stdout, cmd.Stderr = out, out
		err := cmd.Run()
		if ctx.Err() != nil {
			return -1, testContextError(ctx, s.timeout)
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode(), nil
		}
		if err != nil {
			return -1, err
		}
		return 0, nil
	}
	token := strconv.FormatInt(time.Now().UnixNano(), 36)
	launchTimeout := s.launchTimeout
	if launchTimeout <= 0 {
		launchTimeout = simulatorLaunchTimeout
	}
	// The launch and the test each have a deadline of their own: the launch's runs until the host's start mark, and the test's from there.
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	launchTimer := time.AfterFunc(launchTimeout, func() { cancel(errSimulatorLaunchTimeout) })
	defer launchTimer.Stop()
	var testTimer *time.Timer
	cmd := exec.CommandContext(runCtx, "xcrun", append([]string{"simctl", "launch", "--console-pty",
		"--terminate-running-process", s.udid, s.id}, args...)...)
	cmd.Env = os.Environ()
	for _, kv := range env {
		cmd.Env = append(cmd.Env, "SIMCTL_CHILD_"+kv)
	}
	cmd.Env = append(cmd.Env, "SIMCTL_CHILD_NSGO_TEST_RUN="+token)
	logs, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	defer logs.Close()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	var captured bytes.Buffer
	ended := false
	code, readErr := readMarkedRun(io.TeeReader(logs, &captured), token, out, func() error {
		ended = true
		return errors.New("the simulator app ended without its exit code")
	}, func() {
		launchTimer.Stop()
		if s.timeout > 0 && testTimer == nil {
			testTimer = time.AfterFunc(s.timeout, func() { cancel(errSimulatorTestTimeout) })
		}
	})
	launchTimer.Stop()
	if testTimer != nil {
		testTimer.Stop()
	}
	if !ended {
		// The host's result is final. CoreSimulator can keep its console bridge alive afterwards; it must not consume the test's deadline.
		cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	started := bytes.Contains(captured.Bytes(), []byte("nsgo-test-start "+token))
	if readErr != nil && !started {
		// readMarkedRun passes on only the run's own output; before the host started, the launcher's diagnostics are all there is.
		out.Write(captured.Bytes())
	}
	if ended && runCtx.Err() != nil {
		s.stop()
		switch cause := context.Cause(runCtx); {
		case errors.Is(cause, errSimulatorTestTimeout):
			return -1, timeoutError(s.timeout)
		case errors.Is(cause, errSimulatorLaunchTimeout):
			return -1, fmt.Errorf("launch simulator app: the test host did not start within %v", launchTimeout)
		}
		return -1, testContextError(ctx, s.timeout)
	}
	if ended && !started {
		if waitErr != nil {
			return -1, fmt.Errorf("launch simulator app: %w", waitErr)
		}
		return -1, errors.New("the simulator app ended before the test host started: TestMain must call nsgotest.Main")
	}
	// A fast app may report its result before simctl fails to attach, and a crashed one leaves no exit mark whatever simctl's status.
	return code, readErr
}

// iosDeviceTestRunner signs the test build and installs it on an iPhone (-device's, else the only one connected), as nsgo dev -target ios does, and launches it with devicectl for each test, the test's environment as DEVICECTL_CHILD_ variables. devicectl bridges the app's standard output but not its exit code, so the test host marks this run's output and its code with the run's token.
func iosDeviceTestRunner(ctx context.Context, p *project, sys *appleOS, dir string, opts testOptions) (targetRunner, error) {
	device, err := selectIOSDevice(ctx, sys, opts.device)
	if err != nil {
		return targetRunner{}, err
	}
	team, err := iosTeam(ctx, opts.team)
	if err != nil {
		return targetRunner{}, err
	}
	app, err := buildIOSApp(ctx, p, sys, filepath.Join(dir, "app"), false)
	if err != nil {
		return targetRunner{}, err
	}
	if err := installOnIOSDevice(ctx, p, sys, app, filepath.Join(dir, "sign"), team, device); err != nil {
		return targetRunner{}, err
	}
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			token := strconv.FormatInt(time.Now().UnixNano(), 36)
			cmd := exec.CommandContext(ctx, "xcrun", append([]string{"devicectl", "device", "process", "launch", "--console",
				// After --, devicectl takes the test's flags, as -test.run, for the app's rather than its own.
				"--terminate-existing", "--device", device.UDID, p.ID, "--"}, args...)...)
			cmd.Env = append(os.Environ(), "DEVICECTL_CHILD_NSGO_TEST_RUN="+token)
			for _, kv := range env {
				cmd.Env = append(cmd.Env, "DEVICECTL_CHILD_"+kv)
			}
			logs, err := cmd.StdoutPipe()
			if err != nil {
				return -1, err
			}
			cmd.Stderr = io.Discard
			if err := cmd.Start(); err != nil {
				return -1, err
			}
			defer cmd.Wait()
			return readMarkedRun(logs, token, out, func() error {
				if ctx.Err() != nil {
					return testContextError(ctx, opts.timeout)
				}
				return fmt.Errorf("the app on %s ended without its exit code", device.Name)
			}, nil)
		},
		close: func() {},
	}, nil
}

// readMarkedRun copies to out the lines a test host writes between its run's start and exit marks, calls onStart, when it is not nil, at the start mark, and returns the code the exit mark gives, a marked runtime error even before the host starts, or ended's error when the lines end before it.
func readMarkedRun(lines io.Reader, token string, out io.Writer, ended func() error, onStart func()) (int, error) {
	sc := bufio.NewScanner(lines)
	sc.Buffer(make([]byte, 1<<16), 1<<22)
	started := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case strings.HasPrefix(line, "nsgo-test-error "+token+" "):
			return -1, errors.New(strings.TrimPrefix(line, "nsgo-test-error "+token+" "))
		case line == "nsgo-test-start "+token:
			started = true
			if onStart != nil {
				onStart()
			}
		case !started:
		case strings.HasPrefix(line, "nsgo-test-exit "+token+" "):
			code, err := strconv.Atoi(strings.TrimPrefix(line, "nsgo-test-exit "+token+" "))
			if err != nil {
				return -1, fmt.Errorf("the test host's exit mark %q has no code", line)
			}
			return code, nil
		default:
			io.WriteString(out, line+"\n")
		}
	}
	return -1, ended()
}

// androidTestRunner installs the test build on the device (-device's,
// else ANDROID_SERIAL's, else the only one connected) and starts its
// activity for each test, the test's arguments and environment in the
// intent; the app's output comes back through the device's log.
func androidTestRunner(ctx context.Context, p *project, dir string, opts testOptions) (targetRunner, error) {
	test := testProject(p)
	d, err := adbDevice(ctx, opts.device)
	if err != nil {
		return targetRunner{}, err
	}
	if err := d.prepareTestScreen(ctx); err != nil {
		return targetRunner{}, err
	}
	goarch, err := d.goarch(ctx)
	if err != nil {
		return targetRunner{}, err
	}
	apk, err := buildAndroidAPK(ctx, test, dir, goarch)
	if err != nil {
		return targetRunner{}, err
	}
	if err := d.run(ctx, "install", "-r", apk); err != nil {
		return targetRunner{}, err
	}
	stop := func() {
		d.run(context.Background(), "shell", "am", "force-stop", test.ID)
		d.run(context.Background(), "uninstall", test.ID)
	}
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			if err := d.prepareTestScreen(ctx); err != nil {
				return -1, err
			}
			stop()
			d.run(ctx, "logcat", "-c")
			token := strconv.FormatInt(time.Now().UnixNano(), 36)
			envMap := map[string]string{"NSGO_TEST_RUN": token}
			for _, kv := range env {
				k, v, _ := strings.Cut(kv, "=")
				envMap[k] = v
			}
			payload, _ := json.Marshal(map[string]any{"args": args, "env": envMap})
			cmd := exec.CommandContext(ctx, d.adb, "-s", d.serial, "logcat", "-v", "raw", "-s", "nsgo:I")
			logs, err := cmd.StdoutPipe()
			if err != nil {
				return -1, err
			}
			if err := cmd.Start(); err != nil {
				return -1, err
			}
			defer func() {
				cmd.Process.Kill()
				cmd.Wait()
				stop()
			}()
			if err := d.run(ctx, "shell", "am", "start", "-S", "-n", test.ID+androidComponent,
				"--es", "nsgo.test", base64.StdEncoding.EncodeToString(payload)); err != nil {
				return -1, err
			}
			return readMarkedRun(logs, token, out, func() error {
				if ctx.Err() != nil {
					return testContextError(ctx, opts.timeout)
				}
				return errors.New("the device's log ended")
			}, nil)
		},
		close: stop,
	}, nil
}
