package nsgotest

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

func TestPinProcessStart(t *testing.T) {
	original := core.ProcessStart()
	stamp := time.Date(2024, 4, 1, 4, 0, 0, 123, time.FixedZone("fixture", 3600))
	t.Run("first render", func(t *testing.T) {
		PinProcessStart(t, stamp)
		if got := core.ProcessStart(); got != stamp {
			t.Fatalf("before app start = %v, want %v", got, stamp)
		}
		seen := make(chan time.Time, 1)
		app := Start(t, func(*core.Scope) core.Node {
			seen <- core.ProcessStart()
			return core.View{}
		})
		if got := <-seen; got != stamp {
			t.Errorf("first render = %v, want %v", got, stamp)
		}
		app.Advance(time.Second)
		if got := core.ProcessStart(); got != stamp {
			t.Errorf("after advancing the app's clock = %v, want %v", got, stamp)
		}
	})
	if got := core.ProcessStart(); got != original {
		t.Errorf("after fixture cleanup = %v, want %v", got, original)
	}
	t.Run("zero", func(t *testing.T) {
		PinProcessStart(t, time.Time{})
		if got := core.ProcessStart(); !got.IsZero() {
			t.Errorf("zero pin = %v", got)
		}
	})
	if got := core.ProcessStart(); got != original {
		t.Errorf("after second cleanup = %v, want %v", got, original)
	}
}
