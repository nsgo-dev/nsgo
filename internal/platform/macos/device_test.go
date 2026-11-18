//go:build darwin && !ios

package macos

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDeviceIdentity reads the Mac's own name and model as scutil and sysctl print them: the computer name Sharing shows, and the model identifier.
func TestDeviceIdentity(t *testing.T) {
	name, model := deviceIdentity()
	if out, err := exec.Command("scutil", "--get", "ComputerName").Output(); err != nil {
		t.Logf("scutil: %v", err)
	} else if want := strings.TrimSpace(string(out)); name != want {
		t.Errorf("device name %q, scutil's %q", name, want)
	}
	out, err := exec.Command("sysctl", "-n", "hw.model").Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(string(out)); model != want || model == "" {
		t.Errorf("device model %q, sysctl's %q", model, want)
	}
}
