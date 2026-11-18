//go:build windows

package windows

import (
	"strings"
	"syscall"
	"testing"
)

// TestDeviceIdentity reads the machine's own name and model: the computer name the environment reports is the one Windows names the computer by, and the model is the firmware's unless it is a placeholder.
func TestDeviceIdentity(t *testing.T) {
	name, model := deviceIdentity()
	if want := syscall.UTF16ToString(computerNameNetBIOS()); name == "" || !strings.HasPrefix(strings.ToUpper(name), want) {
		t.Errorf("device name %q, want one Windows names %q", name, want)
	}
	raw := regString(syscall.HKEY_LOCAL_MACHINE, 0, `HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName")
	if model != firmwareModel(raw) {
		t.Errorf("device model %q, firmware's %q", model, raw)
	}
	t.Logf("name %q, model %q", name, model)
}

// computerNameNetBIOS is GetComputerNameW's name, the NetBIOS name Windows derives from the host name by upper-casing and cutting it to 15 characters.
func computerNameNetBIOS() []uint16 {
	buf := make([]uint16, 64)
	size := uint32(len(buf))
	if err := syscall.GetComputerName(&buf[0], &size); err != nil {
		return nil
	}
	return buf[:size]
}

func TestFirmwareModel(t *testing.T) {
	for in, want := range map[string]string{
		"Surface Laptop 5":       "Surface Laptop 5",
		" Virtual Machine \x00":  "Virtual Machine",
		"To Be Filled By O.E.M.": "",
		"System Product Name":    "",
		"Default string":         "",
		"":                       "",
	} {
		if got := firmwareModel(in); got != want {
			t.Errorf("firmwareModel(%q) = %q, want %q", in, got, want)
		}
	}
}
