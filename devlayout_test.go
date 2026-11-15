package nsgo

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// nsgo dev writes -layout's value once it has checked it (cmd/nsgo's layoutFlag), which the client reads as the audit's start.
func TestDevLayoutReadsWhatTheToolWrote(t *testing.T) {
	for value, want := range map[string]core.LayoutAudit{
		"":               {},
		"bounds":         {Bounds: true},
		"grid":           {Grid: 8},
		"grid=4.5":       {Grid: 4.5},
		"bounds,grid=12": {Bounds: true, Grid: 12},
	} {
		if got := parseDevLayout(value); got != want {
			t.Errorf("parseDevLayout(%q) = %+v, want %+v", value, got, want)
		}
	}
}
