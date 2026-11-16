package testhost

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestPinSetsTheTestsDisplaySettings(t *testing.T) {
	platform := driver.Environment{Dark: true, ReduceMotion: true, IncreaseContrast: true, OS: driver.OSMacOS, Locale: "en-US", FontScale: 1}
	got := Pin(platform, &driver.Environment{IncreaseContrast: false})
	if got.Dark || got.ReduceMotion || got.IncreaseContrast || got.OS != driver.OSMacOS || got.Locale != "en-US" || got.FontScale != 1 {
		t.Errorf("pinned off: %+v", got)
	}
	got = Pin(driver.Environment{OS: driver.OSMacOS}, &driver.Environment{Dark: true, ReduceMotion: true, IncreaseContrast: true})
	if !got.Dark || !got.ReduceMotion || !got.IncreaseContrast {
		t.Errorf("pinned on: %+v", got)
	}
}
