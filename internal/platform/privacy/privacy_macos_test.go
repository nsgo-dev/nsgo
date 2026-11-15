//go:build darwin && !ios

package privacy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestBundleDeclares(t *testing.T) {
	bundle := func(name, plist string) string {
		contents := filepath.Join(t.TempDir(), name+".app", "Contents")
		if err := os.MkdirAll(contents, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(contents, "MacOS", name)
	}
	const key = "NSMicrophoneUsageDescription"
	if !bundleDeclares(bundle("With", "<key>NSMicrophoneUsageDescription</key>"), key) {
		t.Error("a declared description was missed")
	}
	if bundleDeclares(bundle("Without", "<key>CFBundleName</key>"), key) {
		t.Error("an app without a description declares one")
	}
	if bundleDeclares("/usr/local/bin/tool", key) {
		t.Error("a bare program declares a description")
	}
}

func TestFromTCC(t *testing.T) {
	for raw, want := range map[int]int{0: driver.PermissionNotAsked, 1: driver.PermissionDenied, 2: driver.PermissionDenied,
		3: driver.PermissionGranted, 4: driver.PermissionDenied, 9: driver.PermissionUnknown} {
		if got := fromTCC(raw); got != want {
			t.Errorf("fromTCC(%d) = %d, want %d", raw, got, want)
		}
	}
}

func TestCentralManagerOptionsTurnThePowerAlertOff(t *testing.T) {
	key := global(coreBluetoothPath, "CBCentralManagerOptionShowPowerAlertKey")
	if key == 0 {
		t.Fatal("CoreBluetooth exports no CBCentralManagerOptionShowPowerAlertKey")
	}
	options := centralManagerOptions()
	defer options.Send(sel("release"))
	value := options.Send(sel("objectForKey:"), key)
	if value == 0 || objc.Send[bool](value, sel("boolValue")) {
		t.Errorf("the power alert option is %#x, want NO", value)
	}
}

func TestStatusAnswersEveryPermission(t *testing.T) {
	for _, name := range []string{driver.PermissionAccessibility, driver.PermissionScreenRecording, driver.PermissionInputMonitoring,
		driver.PermissionFullDiskAccess, driver.PermissionAppManagement, driver.PermissionDeveloperTools, driver.PermissionMicrophone,
		driver.PermissionCamera, driver.PermissionCalendars, driver.PermissionReminders, driver.PermissionBluetooth, driver.PermissionMediaLibrary} {
		if st := Status(name); st < driver.PermissionNotAsked || st > driver.PermissionUnknown {
			t.Errorf("%s: status %d on macOS", name, st)
		}
	}
	if st := Status("nope"); st != driver.PermissionUnavailable {
		t.Errorf("an unknown key: %d", st)
	}
}
