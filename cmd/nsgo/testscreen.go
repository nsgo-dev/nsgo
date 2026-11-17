package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// androidTestScreen is a device's display and keyguard state: secure means the trust manager reports the current user's device locked, a keyguard only a credential dismisses.
type androidTestScreen struct{ awake, locked, secure bool }

// readAndroidTestScreen reads AOSP's power, keyguard and trust dumps. Unknown output is an error rather than permission to run behind a lock screen.
//
// Whether a credential guards the keyguard comes from dumpsys trust's deviceLocked, not from dumpsys window policy's secure: the keyguard delegate starts that field at true and can keep it without a credential (seen on Android 9 and 15 emulators after a cold boot), so a swipe-only lock screen would read as secure.
func readAndroidTestScreen(power, policy, trust string) (androidTestScreen, error) {
	var s androidTestScreen
	var wakeKnown, lockKnown, secureKnown bool
	for _, field := range strings.Fields(power) {
		if value, ok := strings.CutPrefix(field, "mWakefulness="); ok {
			s.awake = value == "Awake"
			wakeKnown = value == "Awake" || value == "Asleep" || value == "Dozing" || value == "Dreaming"
			break // The first is the global state, before individual display groups.
		}
	}
	for _, line := range strings.Split(policy, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || value != "true" && value != "false" {
			continue
		}
		switch key {
		case "showing", "inputRestricted", "mInputRestricted":
			lockKnown = true
			s.locked = s.locked || value == "true"
		}
	}
	for _, line := range strings.Split(trust, "\n") {
		// The current user's line: ` User "Owner" (id=0, flags=0x13) (current): trusted=0, trustManaged=0, deviceLocked=1, ...`.
		_, state, ok := strings.Cut(line, "(current):")
		if !ok {
			continue
		}
		for _, field := range strings.Split(state, ",") {
			value, ok := strings.CutPrefix(strings.TrimSpace(field), "deviceLocked=")
			if ok && (value == "1" || value == "0") {
				secureKnown = true
				s.secure = value == "1"
			}
		}
		break
	}
	if !wakeKnown || !lockKnown || !secureKnown {
		return s, errors.New("could not read the screen/keyguard state from dumpsys power, dumpsys window policy and dumpsys trust")
	}
	return s, nil
}

// prepareAndroidTestScreen wakes a sleeping display and dismisses only a keyguard without a credential. The commands use no persistent settings.
func prepareAndroidTestScreen(ctx context.Context, command func(context.Context, ...string) (string, error)) error {
	notReady := func() error {
		return fmt.Errorf("screen did not become awake and unlocked; unlock the device before running tests: %w", ctx.Err())
	}
	// A command the deadline or a cancellation killed says the wait ended, not that adb failed.
	read := func(what string, args ...string) (string, error) {
		out, err := command(ctx, args...)
		if err != nil && ctx.Err() != nil {
			return "", notReady()
		}
		if err != nil {
			return "", fmt.Errorf("read %s: %w", what, err)
		}
		return out, nil
	}
	woke, dismissed := false, false
	for {
		power, err := read("display state", "shell", "dumpsys", "power")
		if err != nil {
			return err
		}
		policy, err := read("keyguard state", "shell", "dumpsys", "window", "policy")
		if err != nil {
			return err
		}
		trust, err := read("credential lock state", "shell", "dumpsys", "trust")
		if err != nil {
			return err
		}
		s, err := readAndroidTestScreen(power, policy, trust)
		if err != nil {
			return err
		}
		if s.locked && s.secure {
			return errors.New("screen is locked with a credential; unlock the device before running tests")
		}
		if s.awake && !s.locked {
			return nil
		}
		if !s.awake && !woke {
			if _, err := command(ctx, "shell", "input", "keyevent", "KEYCODE_WAKEUP"); err != nil {
				return fmt.Errorf("wake display: %w", err)
			}
			woke = true
		}
		if s.awake && s.locked && !dismissed {
			if _, err := command(ctx, "shell", "wm", "dismiss-keyguard"); err != nil {
				return fmt.Errorf("dismiss keyguard; unlock the device before running tests: %w", err)
			}
			dismissed = true
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return notReady()
		case <-timer.C:
		}
	}
}

// androidTestScreenWait bounds preparing the screen, adb's reads included: a loaded emulator can take seconds to answer dumpsys just after it wakes.
const androidTestScreenWait = 15 * time.Second

func (d device) prepareTestScreen(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, androidTestScreenWait)
	defer cancel()
	if err := prepareAndroidTestScreen(ctx, d.output); err != nil {
		return fmt.Errorf("Android test device %s: %w", d.serial, err)
	}
	return nil
}
