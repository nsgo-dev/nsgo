//go:build linux && !android

package linux

import "testing"

func TestPrettyHostname(t *testing.T) {
	for _, c := range []struct{ file, want string }{
		{"PRETTY_HOSTNAME=Ada's laptop\n", "Ada's laptop"},
		{"CHASSIS=laptop\nPRETTY_HOSTNAME=\"Ada's \\\"work\\\" laptop\"\nICON_NAME=computer-laptop\n", `Ada's "work" laptop`},
		{"PRETTY_HOSTNAME='Studio'\n", "Studio"},
		{"  PRETTY_HOSTNAME = Lab\r\n", "Lab"},
		{"# PRETTY_HOSTNAME=commented\nCHASSIS=desktop\n", ""},
		{"", ""},
	} {
		if got := prettyHostname(c.file); got != c.want {
			t.Errorf("prettyHostname(%q) = %q, want %q", c.file, got, c.want)
		}
	}
}

func TestFirmwareModel(t *testing.T) {
	for in, want := range map[string]string{
		"XPS 13 9310\n":                      "XPS 13 9310",
		"Raspberry Pi 4 Model B Rev 1.4\x00": "Raspberry Pi 4 Model B Rev 1.4",
		"To Be Filled By O.E.M.\n":           "",
		"System Product Name\n":              "",
		"Default string\n":                   "",
		"":                                   "",
	} {
		if got := firmwareModel(in); got != want {
			t.Errorf("firmwareModel(%q) = %q, want %q", in, got, want)
		}
	}
}
