package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type androidTestScreen struct{ awake, locked, secure bool }

// readAndroidTestScreen reads AOSP's power and keyguard dumps. Unknown output is an error rather than permission to run behind a lock screen.
func readAndroidTestScreen(power, policy string) (androidTestScreen, error) {
	var s androidTestScreen
	var wakeKnown, lockKnown bool
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
		case "secure":
			s.secure = value == "true"
		}
	}
	if !wakeKnown || !lockKnown {
		return s, errors.New("could not read the screen/keyguard state from dumpsys power and dumpsys window policy")
	}
	return s, nil
}

// prepareAndroidTestScreen wakes a sleeping display and dismisses only a keyguard without a credential. The commands use no persistent settings.
func prepareAndroidTestScreen(ctx context.Context, command func(context.Context, ...string) (string, error)) error {
	woke, dismissed := false, false
	for {
		power, err := command(ctx, "shell", "dumpsys", "power")
		if err != nil {
			return fmt.Errorf("read display state: %w", err)
		}
		policy, err := command(ctx, "shell", "dumpsys", "window", "policy")
		if err != nil {
			return fmt.Errorf("read keyguard state: %w", err)
		}
		s, err := readAndroidTestScreen(power, policy)
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
			return fmt.Errorf("screen did not become awake and unlocked; unlock the device before running tests: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func (d device) prepareTestScreen(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := prepareAndroidTestScreen(ctx, d.output); err != nil {
		return fmt.Errorf("Android test device %s: %w", d.serial, err)
	}
	return nil
}
