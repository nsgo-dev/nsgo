//go:build windows

package windows

import (
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// heldBySomeone reports whether another hot key already holds key's combination: RegisterHotKey refuses a combination any window or thread on the desktop has registered.
func heldBySomeone(t *testing.T, key string) bool {
	t.Helper()
	mods, vk, ok := globalHotKey(key)
	if !ok {
		t.Fatalf("globalHotKey(%q) failed", key)
	}
	const probe = 0xBFFF // the last ID an application may use
	if r, _, _ := registerHotKey.Call(0, probe, uintptr(mods|modNoRepeat), vk); r != 0 {
		unregisterHotKey.Call(0, probe)
		return false
	}
	return true
}

// waitForHeld waits until key's combination is held, or free.
func waitForHeld(t *testing.T, key string, held bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for heldBySomeone(t, key) != held {
		if time.Now().After(deadline) {
			t.Fatalf("%s held = %v, want %v", key, !held, held)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitForRegistered waits until the backend holds exactly the shortcuts ids, as the window's thread sees them. Probe a combination only after this: a probe's RegisterHotKey while the backend is still registering the combination would take it from the backend.
func waitForRegistered(t *testing.T, hwnd uintptr, ids ...uint32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var got []uint32
		onMain(t, hwnd, func(w *window) {
			for id := range w.hotKeys {
				got = append(got, id)
			}
		})
		slices.Sort(got)
		if slices.Equal(got, ids) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("registered shortcuts %v, want %v", got, ids)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestGlobalHotKeysReachTheApp runs an app the way nsgo.Run runs an app function, on a hidden root window, and registers global shortcuts through the core: the backend holds their combinations, WM_HOTKEY runs the shortcut, dropping one releases its combination while the other stays, and closing the app releases the rest.
func TestGlobalHotKeysReachTheApp(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	// The probes register hot keys for this thread, so they must stay on it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const kept, dropped = "ctrl+alt+shift+f23", "ctrl+alt+shift+f24"
	if heldBySomeone(t, kept) || heldBySomeone(t, dropped) {
		t.Skip("another app holds the test's shortcuts")
	}
	title := "nsgo-native-hotkeys"
	fired := make(chan string, 8)
	app := core.AppRoot(func(cx *core.Scope) {
		// The core numbers shortcuts from 1 in the order they register.
		temp := cx.Child()
		cx.RegisterHotKey(kept, func() {
			fired <- kept
			temp.Dispose()
		})
		// Registered while the app function runs, the shortcut would belong to the running owner; Run makes temp own it, so disposing temp drops it.
		temp.Run(func() { temp.RegisterHotKey(dropped, func() { fired <- dropped }) })
	})
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 360, Height: 240, Hidden: true})
	}()
	hwnd := waitForWindow(t, title, done)
	// The window has its title as it is created, before Run records it; a sent message returns only once its thread takes messages, by when it has.
	sendMessage.Call(hwnd, 0, 0, 0)
	closed := false
	t.Cleanup(func() {
		if closed {
			return
		}
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	waitForRegistered(t, hwnd, 1, 2)
	waitForHeld(t, kept, true)
	waitForHeld(t, dropped, true)

	next := func() string {
		t.Helper()
		select {
		case key := <-fired:
			return key
		case <-time.After(5 * time.Second):
			t.Fatal("no shortcut ran")
			return ""
		}
	}
	postMessageW.Call(hwnd, wmHotKey, 2, 0)
	if got := next(); got != dropped {
		t.Fatalf("WM_HOTKEY 2 ran %q, want %q", got, dropped)
	}
	// The kept shortcut drops the other, which the backend unregisters.
	postMessageW.Call(hwnd, wmHotKey, 1, 0)
	if got := next(); got != kept {
		t.Fatalf("WM_HOTKEY 1 ran %q, want %q", got, kept)
	}
	waitForRegistered(t, hwnd, 1)
	waitForHeld(t, dropped, false)
	waitForHeld(t, kept, true)
	// An ID no longer registered runs nothing; messages arrive in order, so the next shortcut to run is the kept one.
	postMessageW.Call(hwnd, wmHotKey, 2, 0)
	postMessageW.Call(hwnd, wmHotKey, 1, 0)
	if got := next(); got != kept {
		t.Fatalf("after the drop the next shortcut was %q, want %q", got, kept)
	}

	postMessageW.Call(hwnd, wmClose, 0, 0)
	select {
	case <-done:
		closed = true
	case <-time.After(5 * time.Second):
		t.Fatal("the window did not close")
	}
	waitForHeld(t, kept, false)
}

// TestGlobalHotKeyAnswers runs an app the way nsgo.Run runs an app function while this thread, standing for another app, holds one of its combinations: that shortcut is taken, a free one registers, and a key RegisterHotKey has no code for is unsupported. Once the other holder lets go, the next change to the app's shortcuts registers the combination, and a second registration of it in the app is taken.
func TestGlobalHotKeyAnswers(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	// The probe's hot key belongs to this thread, which must keep it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const held, free = "ctrl+alt+shift+f21", "ctrl+alt+shift+f22"
	if heldBySomeone(t, held) || heldBySomeone(t, free) {
		t.Skip("another app holds the test's shortcuts")
	}
	mods, vk, _ := globalHotKey(held)
	const probe = 0xBFFE
	if r, _, err := registerHotKey.Call(0, probe, uintptr(mods|modNoRepeat), vk); r == 0 {
		t.Fatalf("the probe could not hold %s: %v", held, err)
	}
	released := false
	release := func() {
		if !released {
			unregisterHotKey.Call(0, probe)
			released = true
		}
	}
	defer release()

	title := "nsgo-native-hotkey-answers"
	answers := make(chan string, 32)
	app := core.AppRoot(func(cx *core.Scope) {
		watch := func(name string, h *core.HotKey) {
			cx.Effect(func() { answers <- name + " " + h.State().String() })
		}
		again := cx.Child()
		watch("held", cx.RegisterHotKey(held, nil))
		watch("unknown", cx.RegisterHotKey("ctrl+alt+shift+nosuchkey", nil))
		// The free shortcut registers the held combination a second time, which sends the set again.
		watch("free", cx.RegisterHotKey(free, func() {
			again.Run(func() { watch("again", again.RegisterHotKey(held, nil)) })
		}))
	})
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 360, Height: 240, Hidden: true})
	}()
	hwnd := waitForWindow(t, title, done)
	sendMessage.Call(hwnd, 0, 0, 0)
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	latest := map[string]string{}
	waitFor := func(want map[string]string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			matched := true
			for name, state := range want {
				matched = matched && latest[name] == state
			}
			if matched {
				return
			}
			select {
			case a := <-answers:
				name, state, _ := strings.Cut(a, " ")
				latest[name] = state
			case <-deadline:
				t.Fatalf("answers %v, want %v", latest, want)
			}
		}
	}
	waitFor(map[string]string{"held": "taken", "unknown": "unsupported", "free": "registered"})

	release()
	// Shortcut 3 is the free one: the core numbers shortcuts from 1 in the order they register.
	postMessageW.Call(hwnd, wmHotKey, 3, 0)
	waitFor(map[string]string{"held": "registered", "again": "taken", "free": "registered", "unknown": "unsupported"})
	waitForHeld(t, held, true)
}
