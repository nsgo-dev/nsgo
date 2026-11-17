//go:build !js

package pkg

import (
	"os"
	"path/filepath"
	"testing"
)

func trustTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// blockTrustWrites keeps the durable state readable while writes fail.
func blockTrustWrites(t *testing.T, dir string) func() {
	t.Helper()
	path := filepath.Join(dir, stateFile+".tmp")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	blocked := true
	unblock := func() {
		t.Helper()
		if blocked {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			blocked = false
		}
	}
	t.Cleanup(unblock)
	return unblock
}
