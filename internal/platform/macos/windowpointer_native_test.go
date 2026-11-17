//go:build darwin && !ios

package macos

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// A window that takes no pointer ignores AppKit's mouse events, and one that takes it where it draws finds a painted view under a point and nothing beside it; switching back gives it the pointer again. Run owns AppKit's process lifetime, so this test gets a process of its own.
func TestWindowPointerEventsFollowTheMode(t *testing.T) {
	if os.Getenv("NSGO_TEST_POINTER_PROCESS") != t.Name() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_POINTER_PROCESS="+t.Name(), "HOME="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	const title = "nsgo-window-pointer"
	done := make(chan error, 1)
	go func() {
		// main finds the window and its pet once laid out.
		main := func() (s *surface, pet *view) {
			for s := range surfaces {
				if s.window == 0 || apple.GoString(s.window.Send(sel("title"))) != title {
					continue
				}
				for _, v := range s.views {
					if v != nil && v.props.TestID == "pet" && v.framed {
						return s, v
					}
				}
			}
			return nil, nil
		}
		check := func() error {
			deadline := time.Now().Add(5 * time.Second)
			for {
				var ready bool
				apple.OnMain(func() { s, _ := main(); ready = s != nil })
				barrier := make(chan struct{})
				apple.OnMain(func() { close(barrier) })
				<-barrier
				if ready {
					break
				}
				if time.Now().After(deadline) {
					return errors.New("the window never laid its pet out")
				}
				time.Sleep(10 * time.Millisecond)
			}
			var err error
			barrier := make(chan struct{})
			apple.OnMain(func() {
				defer close(barrier)
				s, pet := main()
				ignores := func() bool { return objc.Send[bool](s.window, sel("ignoresMouseEvents")) }
				s.setPointerEvents(driver.WindowPointerNone)
				if !ignores() {
					err = errors.New("a window that takes no pointer takes AppKit's mouse events")
					return
				}
				s.setPointerEvents(driver.WindowPointerPainted)
				if s.pointer.timer == 0 {
					err = errors.New("a painted window does not follow the pointer")
					return
				}
				toWindow := sel("convertPoint:toView:")
				onPet := objc.Send[apple.CGPoint](pet.native, toWindow, apple.CGPoint{X: 100, Y: 100}, objc.ID(0))
				beside := objc.Send[apple.CGPoint](s.native, toWindow, apple.CGPoint{X: 500, Y: 300}, objc.ID(0))
				if !s.paintedAt(onPet) {
					err = errors.New("a painted window finds nothing painted over its pet")
					return
				}
				if s.paintedAt(beside) {
					err = errors.New("a painted window finds something painted where nothing draws")
					return
				}
				s.setPointerEvents(driver.WindowPointerAuto)
				if ignores() || s.pointer.timer != 0 {
					err = errors.New("a window given the pointer back still lets it through")
				}
			})
			<-barrier
			return err
		}
		done <- check()
		apple.OnMain(finish)
	}()
	onMain(func() {
		Run(core.AppRoot(func(cx *core.Scope) {
			cx.SetMainView(func(cx *core.Scope) core.Node {
				desktop.SetTitle(cx, title)
				return core.View{Style: core.Style{Width: core.Pt(600), Height: core.Pt(400)}, Children: []core.Node{
					core.View{TestID: "pet", Style: core.Style{Width: core.Pt(200), Height: core.Pt(200), Background: core.RGB(0xcc6600)}},
				}}
			})
		}), Options{Title: title, Width: 600, Height: 400})
	})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
