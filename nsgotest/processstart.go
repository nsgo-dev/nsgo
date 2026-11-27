package nsgotest

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/processstart"
)

// PinProcessStart makes core.ProcessStart return start until t's cleanup,
// including before the app starts. It changes the whole test process, not
// an app's clock. Do not use it in parallel tests or while another pin is
// active. Tests that do not pin keep the real process start time.
func PinProcessStart(t testing.TB, start time.Time) {
	t.Helper()
	release := processstart.Pin(start)
	if release == nil {
		t.Fatal("nsgotest: another PinProcessStart is active; tests pinning the process start cannot overlap")
		return
	}
	t.Cleanup(release)
}
