//go:build ios && nsgotest

package watchos_test

import (
	"testing"
	"time"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/watchos"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

// TestBoundValueReachesTheViewWithoutAnimating mounts a view bound to an animated value that never runs, as a checked switch's knob is: no host frame applies it, so the commit must (#815).
func TestBoundValueReachesTheViewWithoutAnimating(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		x := core.NewAnimatedValue(cx, 12)
		return core.View{TestID: "moved", Style: core.Style{Width: core.Pt(20), Height: core.Pt(20), Animated: &core.AnimatedStyle{TranslateX: x}}}
	})
	app.WaitFor("id:moved", 5*time.Second)
	app.WaitUntil(5*time.Second, func() bool {
		v, ok := watchos.AnimatedForTest("moved")
		return ok && v.TranslateX == 12
	})
}
