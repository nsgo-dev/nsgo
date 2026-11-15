package nsgotest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// MainFunc runs its function to the end, deferred calls included, before the process ends with the function's code. The test runs itself again as a process that calls MainFunc, as a TestMain would, and reads what that process wrote.
func TestMainFuncEndsAfterItsTeardown(t *testing.T) {
	if mark := os.Getenv("NSGO_MAINFUNC_MARK"); mark != "" {
		write := func(s string) {
			f, err := os.OpenFile(mark, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			if _, err := f.WriteString(s + "\n"); err != nil {
				panic(err)
			}
		}
		MainFunc(func() int {
			defer write("teardown")
			write("tests")
			return 17
		})
		t.Fatal("MainFunc returned")
	}
	switch runtime.GOOS {
	case "js", "wasip1", "android", "ios":
		t.Skip("the platform cannot start the test binary again")
	}
	mark := filepath.Join(t.TempDir(), "mark")
	if err := os.WriteFile(mark, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainFuncEndsAfterItsTeardown$")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "NSGO_TEST_TARGET=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "NSGO_MAINFUNC_MARK="+mark)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 17 {
		t.Fatalf("the process ended with %v, want exit status 17:\n%s", err, out)
	}
	got, err := os.ReadFile(mark)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "tests\nteardown\n" {
		t.Fatalf("the process wrote %q, want the tests and then the teardown", got)
	}
}
