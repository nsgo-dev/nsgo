package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPrepareAndroidTestScreen(t *testing.T) {
	for _, tc := range []struct {
		name, wake      string
		locked, secure  bool
		fail, wantError string
		wantActions     []string
	}{
		{name: "already ready", wake: "Awake"},
		{name: "sleeping", wake: "Asleep", wantActions: []string{"shell input keyevent KEYCODE_WAKEUP"}},
		{name: "dozing lock", wake: "Dozing", locked: true, wantActions: []string{"shell input keyevent KEYCODE_WAKEUP", "shell wm dismiss-keyguard"}},
		{name: "awake lock", wake: "Awake", locked: true, wantActions: []string{"shell wm dismiss-keyguard"}},
		{name: "unlocked with credential", wake: "Awake", secure: true},
		{name: "PIN locked", wake: "Dozing", locked: true, secure: true, wantError: "locked with a credential"},
		{name: "credential appears on wake", wake: "Asleep", secure: true, wantError: "locked with a credential", wantActions: []string{"shell input keyevent KEYCODE_WAKEUP"}},
		{name: "wake failure", wake: "Asleep", fail: "shell input keyevent KEYCODE_WAKEUP", wantError: "wake display", wantActions: []string{"shell input keyevent KEYCODE_WAKEUP"}},
		{name: "dismiss failure", wake: "Awake", locked: true, fail: "shell wm dismiss-keyguard", wantError: "unlock the device", wantActions: []string{"shell wm dismiss-keyguard"}},
		{name: "power failure", wake: "Awake", fail: "shell dumpsys power", wantError: "read display state"},
		{name: "policy failure", wake: "Awake", fail: "shell dumpsys window policy", wantError: "read keyguard state"},
		{name: "trust failure", wake: "Awake", fail: "shell dumpsys trust", wantError: "read credential lock state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wake, locked := tc.wake, tc.locked
			var actions []string
			command := func(_ context.Context, args ...string) (string, error) {
				cmd := strings.Join(args, " ")
				if !strings.Contains(cmd, "dumpsys") {
					actions = append(actions, cmd)
				}
				if cmd == tc.fail {
					return "", errors.New("adb failed")
				}
				switch cmd {
				case "shell dumpsys power":
					return "  mWakefulness=" + wake, nil
				case "shell dumpsys window policy":
					// The keyguard delegate can report secure=true without a credential, as Android 9 and 15 do after a cold boot; dumpsys trust is what tells them apart.
					return fmt.Sprintf("KeyguardServiceDelegate\n  showing=%t\n  inputRestricted=%t\n  secure=true\n  showingAndNotOccluded=true", locked, locked), nil
				case "shell dumpsys trust":
					return fmt.Sprintf("Trust manager state:\n User \"Owner\" (id=0, flags=0x13) (current): trusted=0, trustManaged=0, deviceLocked=%d, strongAuthRequired=0x0\n", map[bool]int{false: 0, true: 1}[locked && tc.secure]), nil
				case "shell input keyevent KEYCODE_WAKEUP":
					wake = "Awake"
					locked = locked || tc.secure
				case "shell wm dismiss-keyguard":
					locked = false
				default:
					t.Fatalf("unexpected command %q", cmd)
				}
				return "", nil
			}
			err := prepareAndroidTestScreen(t.Context(), command)
			if tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("prepare = %v, want error %q", err, tc.wantError)
			}
			if !slices.Equal(actions, tc.wantActions) {
				t.Errorf("actions %q, want %q", actions, tc.wantActions)
			}
		})
	}
}

func TestAndroidTestRunnerRefusesALockedDeviceBeforeBuilding(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake adb uses a POSIX shell")
	}
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$*\" >> '" + calls + "'\ncase \"$*\" in\n" +
		"devices) printf 'List of devices attached\\nLOCKED1\\tdevice\\n';;\n" +
		"'-s LOCKED1 shell dumpsys power') echo '  mWakefulness=Dozing';;\n" +
		"'-s LOCKED1 shell dumpsys window policy') printf 'showing=true\\ninputRestricted=true\\nsecure=true\\n';;\n" +
		"'-s LOCKED1 shell dumpsys trust') echo ' User \"Owner\" (id=0, flags=0x13) (current): trusted=0, trustManaged=0, deviceLocked=1, strongAuthRequired=0x0';;\n" +
		"*) echo \"unexpected adb $*\" >&2; exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "adb"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ANDROID_SERIAL", "")
	_, err := androidTestRunner(t.Context(), &project{}, t.TempDir(), testOptions{target: "android"})
	if err == nil || !strings.Contains(err.Error(), "LOCKED1") || !strings.Contains(err.Error(), "locked with a credential") {
		t.Fatalf("runner = %v, want the device's credential lock", err)
	}
	log, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if want := "devices\n-s LOCKED1 shell dumpsys power\n-s LOCKED1 shell dumpsys window policy\n-s LOCKED1 shell dumpsys trust\n"; string(log) != want {
		t.Errorf("adb calls:\n%s\nwant only the state reads before building:\n%s", log, want)
	}
}

func TestAndroidTestScreenUsesGlobalWakefulness(t *testing.T) {
	s, err := readAndroidTestScreen("mWakefulness=Awake\nDisplay group: mWakefulness=Asleep", "showing=false", unlockedTrust)
	if err != nil || !s.awake {
		t.Errorf("global wakefulness = %+v, %v", s, err)
	}
}

func TestAndroidTestScreenUnknownState(t *testing.T) {
	for _, tc := range []struct{ power, policy, trust string }{
		{"", "showing=false", unlockedTrust},
		{"mWakefulness=Unknown", "showing=false", unlockedTrust},
		{"mWakefulness=Awake", "showingAndNotOccluded=false", unlockedTrust},
		{"mWakefulness=Awake", "showing=unknown", unlockedTrust},
		{"mWakefulness=Awake", "showing=true", ""},
		{"mWakefulness=Awake", "showing=true", " User \"Guest\" (id=10, flags=0x0): trusted=0, trustManaged=0, deviceLocked=0"},
		{"mWakefulness=Awake", "showing=true", " User \"Owner\" (id=0, flags=0x13) (current): trusted=0, deviceLocked=maybe"},
	} {
		if _, err := readAndroidTestScreen(tc.power, tc.policy, tc.trust); err == nil {
			t.Errorf("accepted unknown state %q / %q / %q", tc.power, tc.policy, tc.trust)
		}
	}
}

func TestAndroidTestScreenWaitCanBeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	command := func(_ context.Context, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "shell dumpsys power":
			return "mWakefulness=Awake", nil
		case "shell dumpsys window policy":
			return "showing=true\nsecure=false", nil
		case "shell dumpsys trust":
			return unlockedTrust, nil
		case "shell wm dismiss-keyguard":
			cancel() // The device accepts the command but never unlocks.
		}
		return "", nil
	}
	if err := prepareAndroidTestScreen(ctx, command); !errors.Is(err, context.Canceled) {
		t.Errorf("prepare = %v, want context cancellation", err)
	}
}

const unlockedTrust = "Trust manager state:\n User \"Owner\" (id=0, flags=0x13) (current): trusted=0, trustManaged=0, deviceLocked=0, strongAuthRequired=0x0\n"

// TestAndroidTestScreenReadsTheCredentialFromTrust uses an Android 9 emulator's dumps with a swipe-only lock screen up and then with a PIN: the keyguard delegate's secure=true is the same in both, and only dumpsys trust's deviceLocked differs.
func TestAndroidTestScreenReadsTheCredentialFromTrust(t *testing.T) {
	policy := "    KeyguardServiceDelegate\n      showing=true\n      showingAndNotOccluded=true\n      inputRestricted=false\n      occluded=false\n      secure=true\n      dreaming=false\n      currentUser=-10000\n      KeyguardStateMonitor\n        mIsShowing=true\n        mSimSecure=false\n        mInputRestricted=true\n        mTrusted=false\n        mCurrentUserId=0\n"
	for _, tc := range []struct {
		trust  string
		secure bool
	}{
		{" User \"Owner\" (id=0, flags=0x13) (current): trusted=0, trustManaged=0, deviceLocked=0, strongAuthRequired=0x0\n   Enabled agents:\n", false},
		{" User \"Owner\" (id=0, flags=0x13) (current): trusted=0, trustManaged=0, deviceLocked=1, strongAuthRequired=0x0\n   Enabled agents:\n", true},
	} {
		s, err := readAndroidTestScreen("  mWakefulness=Asleep\n  mWakefulnessChanging=false\n", policy, "Trust manager state:\n"+tc.trust)
		if err != nil || s.awake || !s.locked || s.secure != tc.secure {
			t.Errorf("state = %+v, %v; want asleep, locked, secure %t", s, err, tc.secure)
		}
	}
}

// TestAndroidTestScreenReadsOutlastingTheWait checks that a dumpsys the deadline kills, as on a loaded emulator just after it wakes, is reported as the screen not becoming ready rather than as adb failing.
func TestAndroidTestScreenReadsOutlastingTheWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	command := func(ctx context.Context, args ...string) (string, error) {
		<-ctx.Done()
		return "", errors.New("signal: killed")
	}
	err := prepareAndroidTestScreen(ctx, command)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "did not become awake and unlocked") {
		t.Errorf("prepare = %v, want the wait's deadline", err)
	}
}
