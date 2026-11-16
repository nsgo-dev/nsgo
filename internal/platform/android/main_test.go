package android

import (
	"os"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// TestMain hands the main thread to the Android test host, which this package registers when built for nsgo test -target android, and runs the tests plainly anywhere else. The tool looks for it on this machine's view of the package, so it stays in an untagged file.
func TestMain(m *testing.M) {
	if h := testhost.Current(); h != nil {
		h.Main(m.Run)
		return
	}
	os.Exit(m.Run())
}
