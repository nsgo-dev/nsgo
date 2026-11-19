package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

func testhostOptions(observe func(uint32, *driver.MountBatch)) testhost.Options {
	return testhost.Options{Observe: observe}
}

func TestUseTestKeepsCommitChecksPerEngine(t *testing.T) {
	for _, checked := range []bool{false, true, false, true} {
		name := "measured"
		if checked {
			name = "normal"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := Options{CheckCommits: !checked}
			o.UseTest(testhost.Options{CheckCommits: checked}, &TestPin{})
			if o.DataDir != "" {
				t.Cleanup(func() { os.RemoveAll(filepath.Dir(o.DataDir)) })
			}
			if o.CheckCommits != checked {
				t.Fatalf("test host did not give its engine independent checks: %t", o.CheckCommits)
			}
		})
	}
}
