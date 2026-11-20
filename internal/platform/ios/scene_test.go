//go:build ios && nsgotest

package ios_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/ios"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestSceneReconnectKeepsTheEngine disconnects and reconnects the app's one scene, as UIKit may with a scene in the background, and checks that the running engine shows in the new window with its state, rather than a second engine starting the app over it (#1030).
func TestSceneReconnectKeepsTheEngine(t *testing.T) {
	const wait = 5 * time.Second
	var built, presses atomic.Int32
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		built.Add(1)
		count := core.NewSignal(cx, 0)
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			core.View{TestID: "add", Style: core.Style{Width: core.Pt(160), Height: core.Pt(60), Background: core.RGB(0x3366cc)},
				OnPress: func(core.PressEvent) {
					count.Update(func(n int) int { return n + 1 })
					presses.Store(int32(count.Get()))
				}},
			core.Text{TestID: "count", ContentFunc: func() string { return fmt.Sprint(count.Get()) }},
		}}
	})
	app.WaitFor("id:add", wait)
	app.Press("id:add")
	app.WaitUntil(wait, func() bool { return presses.Load() == 1 })
	for round := range 2 {
		kept, shown := ios.SceneReconnectForTest()
		if !kept {
			t.Fatalf("round %d: the reconnected scene started another engine", round)
		}
		if !shown {
			t.Fatalf("round %d: the surface does not show in the reconnected scene's key window", round)
		}
		if n := built.Load(); n != 1 {
			t.Fatalf("round %d: the app's root was built %d times, want once", round, n)
		}
		// The press reaches the app through the new window, with the count it had.
		app.Press("id:add")
		app.WaitUntil(wait, func() bool { return presses.Load() == int32(round+2) })
	}
}
