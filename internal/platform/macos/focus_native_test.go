//go:build darwin && !ios

package macos

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func programmaticFocusApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		var button, plain, explicit, field core.Ref
		focused, pressed := core.NewSignal(cx, "ready"), core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%s|%d", title, focused.Get(), pressed.Get())) })
		cx.OnKeyWith("mod+f", core.KeyOptions{When: core.KeyAlways}, field.Focus)
		cx.OnKeyWith("mod+b", core.KeyOptions{When: core.KeyAlways}, button.Focus)
		cx.OnKeyWith("mod+p", core.KeyOptions{When: core.KeyAlways}, plain.Focus)
		cx.OnKeyWith("mod+v", core.KeyOptions{When: core.KeyAlways}, explicit.Focus)
		cx.OnKeyWith("mod+n", core.KeyOptions{When: core.KeyAlways}, field.Blur)
		cx.OnKeyWith("mod+m", core.KeyOptions{When: core.KeyAlways}, func() { focused.Set(focused.Get() + "-checked") })
		cx.OnKeyWith("mod+q", core.KeyOptions{When: core.KeyAlways}, cx.Quit)
		return core.View{Style: core.Style{Width: core.Pt(200), Height: core.Pt(200)}, Children: []core.Node{
			core.Input{TestID: "field", Ref: &field, Style: core.Style{Height: core.Pt(30)},
				OnFocus: func() { focused.Set("field") }},
			core.View{TestID: "button", Ref: &button, Style: core.Style{Width: core.Pt(100), Height: core.Pt(40),
				Background: core.RGB(0xcc0000), FocusVisible: &core.Style{Background: core.RGB(0x123456)}},
				OnFocus: func() { focused.Set("button") }, OnBlur: func() { focused.Set("blur") },
				OnPress: func(core.PressEvent) { pressed.Update(func(n int) int { return n + 1 }) }},
			core.View{TestID: "explicit", Ref: &explicit, Style: core.Style{Height: core.Pt(30)},
				Focusable: func() bool { return true }, OnFocus: func() { focused.Set("explicit") }},
			core.View{TestID: "plain", Ref: &plain, Style: core.Style{Height: core.Pt(30)}},
			core.View{TestID: "trigger", NoFocus: true, Style: core.Style{Height: core.Pt(30)},
				OnPress: func(core.PressEvent) { button.Focus() }},
		}}
	}
}

// Run owns AppKit's process lifetime, so this test gets a process of its own. The assertions read AppKit's real first responder and the mounted focus style after the engine has handled its native focus reports.
func TestRefFocusUsesNativeKeyboardStops(t *testing.T) {
	if os.Getenv("NSGO_TEST_FOCUS_PROCESS") != t.Name() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_FOCUS_PROCESS="+t.Name(), "HOME="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	const title = "nsgo-ref-focus"
	done := make(chan error, 1)
	go func() {
		var live *surface
		wait := func(want string) error {
			deadline := time.Now().Add(5 * time.Second)
			last := ""
			for time.Now().Before(deadline) {
				var got string
				var seen []string
				apple.OnMain(func() {
					for s := range surfaces {
						if s.window != 0 {
							seen = append(seen, apple.GoString(s.window.Send(sel("title"))))
						}
						if s.window != 0 && strings.HasPrefix(apple.GoString(s.window.Send(sel("title"))), title) {
							live = s
							break
						}
					}
					if live != nil {
						got = apple.GoString(live.window.Send(sel("title")))
					}
				})
				// Synchronize with the main queue before reading got.
				barrier := make(chan struct{})
				apple.OnMain(func() { close(barrier) })
				<-barrier
				last = fmt.Sprintf("current=%q, surfaces=%q", got, seen)
				if got == want {
					return nil
				}
				time.Sleep(10 * time.Millisecond)
			}
			return fmt.Errorf("window did not reach %q: %s", want, last)
		}
		key := func(name string) {
			apple.OnMain(func() {
				if live != nil {
					// The platform reports a key binding by its ID, as it does a matched press.
					for _, b := range live.keys {
						if b.Key == name {
							live.engine.Dispatch(driver.Event{Kind: driver.EventKey, Count: b.ID})
						}
					}
				} else {
					finish()
				}
			})
		}
		check := func() error {
			if err := wait(title + "|ready|0"); err != nil {
				return err
			}
			key("mod+f")
			if err := wait(title + "|field|0"); err != nil {
				return err
			}
			key("mod+b")
			if err := wait(title + "|button|0"); err != nil {
				return err
			}
			var nativeErr error
			barrier := make(chan struct{})
			apple.OnMain(func() {
				defer close(barrier)
				v := views[live.window.Send(sel("firstResponder"))]
				if v == nil || v.props.TestID != "button" {
					nativeErr = fmt.Errorf("Ref.Focus did not make the button AppKit's first responder")
					return
				}
				live.engine.InputModality(driver.ModalityKeyboard)
				for _, k := range []struct {
					code uint16
					text string
				}{{49, " "}, {36, "\r"}} {
					for _, repeat := range []uint64{0, 1} {
						event := apple.KeyEvent(evKeyDown, 0, 0, objc.Send[int](live.window, sel("windowNumber")), k.text, k.text, repeat != 0, k.code)
						v.native.Send(sel("keyDown:"), event)
					}
				}
			})
			<-barrier
			if nativeErr != nil {
				return nativeErr
			}
			if err := wait(title + "|button|2"); err != nil {
				return err
			}
			barrier = make(chan struct{})
			apple.OnMain(func() {
				defer close(barrier)
				v := views[live.window.Send(sel("firstResponder"))]
				if v == nil || v.style.Background != driver.RGB(0x123456) {
					nativeErr = fmt.Errorf("the focused button did not receive its focus-visible style")
				}
			})
			<-barrier
			if nativeErr != nil {
				return nativeErr
			}
			key("mod+p")
			key("mod+n")
			key("mod+m")
			if err := wait(title + "|button-checked|2"); err != nil {
				return err
			}
			key("mod+v")
			return wait(title + "|explicit|2")
		}
		err := check()
		done <- err
		key("mod+q")
	}()
	onMain(func() {
		Run(core.AppRoot(func(cx *core.Scope) { cx.SetMainView(programmaticFocusApp(title)) }),
			Options{Title: title, Width: 200, Height: 200})
	})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
