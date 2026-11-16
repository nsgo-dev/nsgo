//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// TestHotKeyRegistrationAnswers registers global shortcuts with Carbon and checks each one's answer: a combination registers, the same one again is taken, a key without a virtual key code is unsupported, and the combination registers again once the set that held it is gone.
func TestHotKeyRegistrationAnswers(t *testing.T) { onMain(func() { hotKeyRegistrationAnswers(t) }) }

func hotKeyRegistrationAnswers(t *testing.T) {
	answers := map[uint32]driver.HotKeyState{}
	eng := engine.Start(actionEngine{}, func(*core.Scope) core.Node { return core.View{} }, engine.Options{Event: func(e driver.Event) {
		if e.Kind == driver.EventHotKeyState {
			answers[e.Count] = driver.HotKeyState(e.Index)
		}
	}})
	defer eng.Close()
	s := &surface{engine: eng}
	defer s.hotKeys(nil)
	const combo = "ctrl+alt+shift+f19"
	s.hotKeys([]driver.KeyBinding{{Key: combo, ID: 1}, {Key: combo, ID: 2}, {Key: "ctrl+alt+shift+f21", ID: 3}})
	want := map[uint32]driver.HotKeyState{1: driver.HotKeyRegistered, 2: driver.HotKeyTaken, 3: driver.HotKeyUnsupported}
	for id, state := range want {
		if answers[id] != state {
			t.Errorf("shortcut %d answered %v, want %v", id, answers[id], state)
		}
	}
	if len(registered) != 1 {
		t.Errorf("%d hot keys registered, want 1", len(registered))
	}
	s.hotKeys([]driver.KeyBinding{{Key: combo, ID: 4}})
	if answers[4] != driver.HotKeyRegistered {
		t.Errorf("the combination answered %v once the set holding it was replaced, want registered", answers[4])
	}
	s.hotKeys(nil)
	if len(registered) != 0 {
		t.Errorf("%d hot keys stay registered without shortcuts", len(registered))
	}
}
