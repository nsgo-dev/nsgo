//go:build darwin

package apple

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// step is one thing that happens to a video's player, and what the backend reports for it.
type step struct {
	name string
	do   func(p *videoPlayback) (driver.Event, bool)
	want []driver.Event
	// apply is whether a Configure that follows gives the player the app's play state.
	apply bool
}

func pausedFor(reason int) driver.Event {
	return driver.Event{Kind: driver.EventVideoPause, Index: reason}
}

var played = driver.Event{Kind: driver.EventVideoPlay}

func runSteps(t *testing.T, steps []step) {
	t.Helper()
	var p videoPlayback
	for _, s := range steps {
		var got []driver.Event
		if ev, ok := s.do(&p); ok {
			got = append(got, ev)
		}
		if len(got) != len(s.want) || len(got) == 1 && (got[0].Kind != s.want[0].Kind || got[0].Index != s.want[0].Index) {
			t.Fatalf("%s: reported %v, want %v", s.name, got, s.want)
		}
		if apply := !p.held; apply != s.apply {
			t.Fatalf("%s: a Configure applies Paused %v, want %v", s.name, apply, s.apply)
		}
	}
}

// configure is a Configure of the app's paused: it gives the player that state when the backend's videoPlayback lets it.
func configure(paused, changed bool) func(p *videoPlayback) (driver.Event, bool) {
	return func(p *videoPlayback) (driver.Event, bool) {
		if p.configure(changed) {
			p.applied(!paused)
		}
		return driver.Event{}, false
	}
}

// reached is the player reaching playing on its own while the app's Paused is paused.
func reached(playing, paused bool, reason int) func(p *videoPlayback) (driver.Event, bool) {
	return func(p *videoPlayback) (driver.Event, bool) { return p.changed(playing, paused, reason) }
}

func TestVideoControlsPauseHoldsUntilPausedChanges(t *testing.T) {
	runSteps(t, []step{
		{name: "opened playing", do: configure(false, true), apply: true},
		// The app's own changes come back as rate notifications that change nothing.
		{name: "own rate", do: reached(true, false, driver.VideoPauseControls), apply: true},
		{name: "controls pause", do: reached(false, false, driver.VideoPauseControls), want: []driver.Event{pausedFor(driver.VideoPauseControls)}},
		{name: "the same rate again", do: reached(false, false, driver.VideoPauseControls)},
		// Muted changes: the pause holds.
		{name: "unrelated config", do: configure(false, false)},
		{name: "app follows", do: configure(true, true), apply: true},
		{name: "app plays", do: configure(false, true), apply: true},
	})
}

func TestVideoControlsPlayWhileTheAppIsPausedIsReported(t *testing.T) {
	runSteps(t, []step{
		{name: "opened paused", do: configure(true, true), apply: true},
		{name: "controls play", do: reached(true, true, driver.VideoPauseControls), want: []driver.Event{played}},
		{name: "unrelated config", do: configure(true, false)},
		{name: "controls pause", do: reached(false, true, driver.VideoPauseControls), want: []driver.Event{pausedFor(driver.VideoPauseControls)}, apply: true},
		{name: "controls play again", do: reached(true, true, driver.VideoPauseControls), want: []driver.Event{played}},
		{name: "app follows", do: configure(false, true), apply: true},
	})
}

func TestVideoSystemPauseKeepsTheReasonItGaveFirst(t *testing.T) {
	runSteps(t, []step{
		{name: "opened playing", do: configure(false, true), apply: true},
		{name: "route lost", do: func(p *videoPlayback) (driver.Event, bool) {
			p.expect(driver.VideoPauseRouteLost)
			return p.changed(false, false, driver.VideoPauseControls)
		}, want: []driver.Event{pausedFor(driver.VideoPauseRouteLost)}},
		// A lost route does not play on by itself.
		{name: "no resume", do: func(p *videoPlayback) (driver.Event, bool) {
			if p.resume() {
				t.Error("a lost route resumed")
			}
			return driver.Event{}, false
		}},
		{name: "controls play", do: reached(true, false, driver.VideoPauseControls), want: []driver.Event{played}, apply: true},
	})
}

func TestVideoInterruptionResumesUnlessTheAppTookIt(t *testing.T) {
	interrupted := reached(false, false, driver.VideoPauseInterruption)
	resume := func(want bool) func(p *videoPlayback) (driver.Event, bool) {
		return func(p *videoPlayback) (driver.Event, bool) {
			if got := p.resume(); got != want {
				t.Errorf("resume() = %v, want %v", got, want)
			}
			if want {
				return p.changed(true, false, driver.VideoPauseInterruption)
			}
			return driver.Event{}, false
		}
	}
	runSteps(t, []step{
		{name: "opened playing", do: configure(false, true), apply: true},
		{name: "interrupted", do: interrupted, want: []driver.Event{pausedFor(driver.VideoPauseInterruption)}},
		{name: "unrelated config", do: configure(false, false)},
		{name: "interruption ends", do: resume(true), want: []driver.Event{played}, apply: true},
		{name: "interrupted again", do: interrupted, want: []driver.Event{pausedFor(driver.VideoPauseInterruption)}},
		{name: "app follows", do: configure(true, true), apply: true},
		{name: "interruption ends after the app paused", do: resume(false), apply: true},
	})
}

func TestVideoEndLeavesThePlayStateToPaused(t *testing.T) {
	runSteps(t, []step{
		{name: "opened playing", do: configure(false, true), apply: true},
		{name: "controls pause", do: reached(false, false, driver.VideoPauseControls), want: []driver.Event{pausedFor(driver.VideoPauseControls)}},
		{name: "controls play", do: reached(true, false, driver.VideoPauseControls), want: []driver.Event{played}, apply: true},
		{name: "ended", do: func(p *videoPlayback) (driver.Event, bool) { p.ended(); return driver.Event{}, false }, apply: true},
		// The rate notification of the end comes after it.
		{name: "end's rate", do: reached(false, false, driver.VideoPauseControls), apply: true},
	})
}
