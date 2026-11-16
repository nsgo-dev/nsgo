//go:build darwin && !ios

package main

import (
	"os"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

// inactive starts the app and deactivates it, as when another app is in front, whichever app that is. Only this test process's app and windows are touched. A test does not deactivate again once -activate activated the app: with no other app to take over, as on a CI runner, AppKit then leaves the app active with a window it can no longer make key.
func inactive(t *testing.T) *nsgotest.App {
	t.Helper()
	if nsgotest.Target() != "macos" {
		t.Skip("requires the macOS native test host")
	}
	a := nsgotest.Start(t, Root)
	deadline := time.Now().Add(2 * time.Second)
	for {
		done := make(chan bool, 1)
		apple.OnMain(func() {
			app := objc.ID(apple.Class("NSApplication")).Send(objc.RegisterName("sharedApplication"))
			// Without -activate (the e2e test sets 0), a prohibited app can never activate, so its field clicks meet an inactive app on every Mac, even one where AppKit would make an inactive window key.
			if os.Getenv("NSGO_TEST_ACTIVATE") == "0" {
				app.Send(objc.RegisterName("setActivationPolicy:"), int(2)) // prohibited
			}
			app.Send(objc.RegisterName("deactivate"))
			w := app.Send(objc.RegisterName("keyWindow"))
			if w != 0 {
				w.Send(objc.RegisterName("resignKeyWindow"))
			}
			done <- !objc.Send[bool](app, objc.RegisterName("isActive")) && app.Send(objc.RegisterName("keyWindow")) == 0
		})
		if <-done {
			return a
		}
		if time.Now().After(deadline) {
			t.Fatal("could not deactivate the test app")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// requireKeyboard fails the test unless the app is active with a key window.
func requireKeyboard(t *testing.T) {
	t.Helper()
	done := make(chan bool, 1)
	apple.OnMain(func() {
		app := objc.ID(apple.Class("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		done <- objc.Send[bool](app, objc.RegisterName("isActive")) && app.Send(objc.RegisterName("keyWindow")) != 0
	})
	if !<-done {
		t.Fatal("the focus step left the native app without the keyboard")
	}
}

func TestFocusAndType(t *testing.T) {
	a := inactive(t)
	a.Focus("field")
	requireKeyboard(t)
	a.Type("field", "Native keyboard")
	if v := a.Find("field"); !v.Focused || v.Text != "Native keyboard" {
		t.Fatalf("field focused=%v text=%q", v.Focused, v.Text)
	}
	// Focusing the focused field again keeps the keyboard.
	a.Focus("field")
	requireKeyboard(t)
	a.Clear("field")
	a.Type("field", "Again")
	a.Find("text:Again")
}

func TestImplicitFocusAndType(t *testing.T) {
	a := inactive(t)
	a.Type("field", "Implicit focus")
	a.Find("text:Implicit focus")
}

func TestTabAndKey(t *testing.T) {
	a := inactive(t)
	a.Focus("button")
	if !a.Find("button").Focused {
		t.Fatal("Tab did not focus the button")
	}
	a.Key("Space")
	a.Find("text:Pressed")
}
