package main

import "testing"

func TestHdcFailure(t *testing.T) {
	for _, c := range []struct {
		out, want string
	}{
		{"[Info]App install path:/x/Gallery.hap msg:install bundle successfully.\nAppMod finish\n", ""},
		{"start ability successfully.\n", ""},
		{"[Info]App install path:/x/Gallery.hap msg:error: failed to install bundle. code:9568320 error: no signature file. \nAppMod finish\n", "failed to install bundle. code:9568320 error: no signature file."},
		{"error: failed to start ability.\nError Code:10104001  Error Message:The specified ability does not exist\n", "failed to start ability."},
	} {
		err := hdcFailure(c.out)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("hdcFailure(%q) = %q, want %q", c.out, got, c.want)
		}
	}
}
