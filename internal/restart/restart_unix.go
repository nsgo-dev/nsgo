//go:build (darwin && !ios) || (linux && !android)

package restart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
)

// Start starts the program again with args once this process has exited, through a shell that waits for it at most 10 seconds: a macOS app in a bundle through Launch Services, as the user opens it.
func Start(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return restartCommand(os.Getpid(), restartProgram(exe, args)).Start()
}

// restartCommand runs program once process pid has exited, or 10 seconds
// on, in a process group of its own.
func restartCommand(pid int, program []string) *exec.Cmd {
	const script = `i=0; while kill -0 "$0" 2>/dev/null && [ $i -lt 100 ]; do sleep 0.1; i=$((i+1)); done; exec "$@"`
	cmd := exec.Command("/bin/sh", append([]string{"-c", script, fmt.Sprint(pid)}, program...)...)
	env := os.Environ()
	// AppImage's FUSE mount disappears with the old process. Starting the image path again makes its runtime mount a fresh copy; stale runtime variables would otherwise point into the unmounted APPDIR.
	if os.Getenv("APPIMAGE") != "" {
		env = slices.DeleteFunc(env, func(value string) bool {
			key, _, _ := strings.Cut(value, "=")
			return key == "APPDIR" || key == "APPIMAGE" || key == "ARGV0"
		})
	}
	cmd.Env = append(env, guardEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// restartProgram is the command line starting exe with args again.
func restartProgram(exe string, args []string) []string {
	if image := appImage(exe); image != "" {
		return append([]string{image}, args...)
	}
	if bundle := appBundle(exe); runtime.GOOS == "darwin" && bundle != "" {
		program := []string{"/usr/bin/open", "-n", bundle}
		if len(args) > 0 {
			program = append(append(program, "--args"), args...)
		}
		return program
	}
	return append([]string{exe}, args...)
}

// appImage is the AppImage exe runs from, or "". The AppImage runtime names the image in APPIMAGE and its mount in APPDIR, and every process an AppImage app starts inherits both, so the image counts only when exe lies inside that mount.
func appImage(exe string) string {
	image, dir := os.Getenv("APPIMAGE"), os.Getenv("APPDIR")
	if !filepath.IsAbs(image) || !filepath.IsAbs(dir) {
		return ""
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	rel, err := filepath.Rel(dir, exe)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return image
}

// appBundle is the .app bundle whose executable exe is, or "".
func appBundle(exe string) string {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || filepath.Ext(bundle) != ".app" {
		return ""
	}
	return bundle
}

// Await only marks a restarted process (Looping): Start's shell waits instead.
func Await() { markRestarted() }
