package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// iosTestRunner installs the test build in sys's simulator (-device's,
// else the booted one, else the newest iPhone or Apple TV) and launches
// it for each test, its output streamed back.
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
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			cmd := exec.CommandContext(ctx, "xcrun", append([]string{"simctl", "launch", "--console-pty",
				"--terminate-running-process", udid, p.ID}, args...)...)
			cmd.Env = os.Environ()
			for _, kv := range env {
				cmd.Env = append(cmd.Env, "SIMCTL_CHILD_"+kv)
			}
			cmd.Stdout, cmd.Stderr = out, out
			err := cmd.Run()
			if ctx.Err() != nil {
				exec.Command("xcrun", "simctl", "terminate", udid, p.ID).Run()
				return -1, testContextError(ctx, opts.timeout)
			}
			if exit, ok := err.(*exec.ExitError); ok {
				return exit.ExitCode(), nil
			}
			return 0, err
		},
		close: func() { exec.Command("xcrun", "simctl", "terminate", udid, p.ID).Run() },
	}, nil
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
			})
		},
		close: func() {},
	}, nil
}

// readMarkedRun copies to out the lines a test host writes between its run's start and exit marks, and returns the code the exit mark gives, a marked runtime error even before the host starts, or ended's error when the lines end before it.
func readMarkedRun(lines io.Reader, token string, out io.Writer, ended func() error) (int, error) {
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
	d, err := adbDevice(ctx, opts.device)
	if err != nil {
		return targetRunner{}, err
	}
	goarch, err := d.goarch(ctx)
	if err != nil {
		return targetRunner{}, err
	}
	apk, err := buildAndroidAPK(ctx, p, dir, goarch)
	if err != nil {
		return targetRunner{}, err
	}
	if err := d.run(ctx, "install", "-r", apk); err != nil {
		return targetRunner{}, err
	}
	stop := func() { d.run(context.Background(), "shell", "am", "force-stop", p.ID) }
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			stop()
			d.run(ctx, "logcat", "-c")
			token := strconv.FormatInt(time.Now().UnixNano(), 36)
			envMap := map[string]string{"NSGO_TEST_RUN": token}
			for _, kv := range env {
				k, v, _ := strings.Cut(kv, "=")
				envMap[k] = v
			}
			test, _ := json.Marshal(map[string]any{"args": args, "env": envMap})
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
			if err := d.run(ctx, "shell", "am", "start", "-S", "-n", p.ID+androidComponent,
				"--es", "nsgo.test", base64.StdEncoding.EncodeToString(test)); err != nil {
				return -1, err
			}
			return readMarkedRun(logs, token, out, func() error {
				if ctx.Err() != nil {
					return testContextError(ctx, opts.timeout)
				}
				return errors.New("the device's log ended")
			})
		},
		close: stop,
	}, nil
}
