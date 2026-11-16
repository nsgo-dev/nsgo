package nsgotest

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

func TestMeasureKeepsOtherAppsCommitChecks(t *testing.T) {
	for _, measured := range []bool{true, false, true, false} {
		t.Run(map[bool]string{true: "measured", false: "normal"}[measured], func(t *testing.T) {
			t.Parallel()
			a := &App{Clock: core.NewManualClock(time.Unix(0, 0))}
			if measured {
				Measure()(a)
			}
			o := a.targetOptions(&target{a: a})
			if o.CheckCommits != (!automationBuild && !measured) {
				t.Errorf("measured=%t: commit checks=%t", measured, o.CheckCommits)
			}
			if (o.Clock == nil) != measured {
				t.Errorf("measured=%t: system clock=%t", measured, o.Clock == nil)
			}
			// Opening a measurement must not change a subsequent app.
			next := (&App{Clock: a.Clock}).targetOptions(&target{})
			if next.CheckCommits != !automationBuild {
				t.Error("a measurement disabled the next app's commit checks")
			}
		})
	}
}
