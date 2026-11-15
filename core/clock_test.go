package core

import (
	"testing"
	"time"
)

func TestManualClockWallTimeKeepsSetLocation(t *testing.T) {
	clock := NewManualClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tokyo := time.FixedZone("JST", 9*60*60)
	want := time.Date(2026, 11, 16, 8, 0, 0, 0, tokyo)

	clock.SetWallTime(want)
	got := clock.WallTime()
	if !got.Equal(want) || got.Location() != tokyo {
		t.Fatalf("WallTime() = %v (%v), want %v (%v)", got, got.Location(), want, want.Location())
	}

	clock.Advance(2 * time.Hour)
	want = want.Add(2 * time.Hour)
	got = clock.WallTime()
	if !got.Equal(want) || got.Location() != tokyo {
		t.Fatalf("WallTime() after Advance = %v (%v), want %v (%v)", got, got.Location(), want, want.Location())
	}
}
