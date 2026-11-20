package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// androidTestScreen is a device's display and keyguard state: secure means the trust manager reports the current user's device locked, a keyguard only a credential dismisses.
type androidTestScreen struct{ awake, locked, secure bool }

// androidTestScreenReadError means adb failed to read one of the three state dumps after the short retry window. Callers use it to stop a target run once with a device-lost error instead of turning every remaining test into the same failure.
type androidTestScreenReadError struct {
	what string
	err  error
}

func (e *androidTestScreenReadError) Error() string { return fmt.Sprintf("read %s: %v", e.what, e.err) }
func (e *androidTestScreenReadError) Unwrap() error { return e.err }

const (
	androidTestScreenReadAttempts = 3
	androidTestScreenRetryDelay   = 100 * time.Millisecond
)

func adbDeviceDisconnected(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 255
}

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
		for attempt := 1; ; attempt++ {
			out, err := command(ctx, args...)
			if err == nil {
				return out, nil
			}
			if ctx.Err() != nil {
				return "", notReady()
			}
			if adbDeviceDisconnected(err) {
				// A transport loss can leave the serial absent for a moment; wait-for-device gives adb a chance to reconnect before retrying.
				if _, waitErr := command(ctx, "wait-for-device"); waitErr != nil {
					if ctx.Err() != nil {
						return "", notReady()
					}
					err = waitErr
				}
			}
			if attempt == androidTestScreenReadAttempts {
				return "", &androidTestScreenReadError{what: what, err: err}
			}
			timer := time.NewTimer(androidTestScreenRetryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", notReady()
			case <-timer.C:
			}
		}
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
		// Only adb failing to answer after its retries means the device is gone; a screen that does not wake or unlock in time still fails just this test, with the unlock hint.
		var readErr *androidTestScreenReadError
		if errors.As(err, &readErr) {
			return &targetDeviceLostError{err: fmt.Errorf("Android test device %s lost: %w", d.serial, err)}
		}
		return fmt.Errorf("Android test device %s: %w", d.serial, err)
	}
	return nil
}
