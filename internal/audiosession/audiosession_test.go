package audiosession

import (
	"slices"
	"sync"
	"testing"
	"time"
)

// move is one call of apply.
type move struct{ from, to Level }

// recordMoves has apply record its moves and the deactivation wait after, restoring both when the test ends, which waits for the session to be back at none.
func recordMoves(t *testing.T, after time.Duration, block func(move)) func() []move {
	t.Helper()
	var mu sync.Mutex
	var moves []move
	oldApply, oldAfter := apply, deactivateAfter
	apply = func(from, to Level) error {
		if block != nil {
			block(move{from, to})
		}
		mu.Lock()
		moves = append(moves, move{from, to})
		mu.Unlock()
		return nil
	}
	deactivateAfter = after
	t.Cleanup(func() {
		waitFor(t, func() bool { return holders.applied == 0 && holders.deactivation == nil })
		apply, deactivateAfter = oldApply, oldAfter
	})
	return func() []move {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(moves)
	}
}

func level() Level {
	holders.Lock()
	defer holders.Unlock()
	return holders.applied
}

func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for range 500 {
		holders.Lock()
		ok := done()
		holders.Unlock()
		if ok {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("the session did not settle")
}

// The highest level held sets the session, and releasing the last goes back to none once the deactivation runs.
func TestHighestHolderWins(t *testing.T) {
	moves := recordMoves(t, time.Millisecond, nil)
	var applied []Level
	video, err := Hold(Playback)
	if err != nil {
		t.Fatal(err)
	}
	applied = append(applied, level())
	record, _ := Hold(Recording)
	applied = append(applied, level())
	sound, _ := Hold(Ambient)
	applied = append(applied, level())
	record()
	record()
	applied = append(applied, level())
	video()
	applied = append(applied, level())
	kept, _ := Hold(Kept)
	sound()
	applied = append(applied, level())
	kept()
	waitFor(t, func() bool { return holders.applied == 0 })
	want := []Level{Playback, Recording, Recording, Playback, Ambient, Kept}
	if !slices.Equal(applied, want) {
		t.Fatalf("applied %v, want %v", applied, want)
	}
	if got, want := moves(), []move{{0, Playback}, {Playback, Recording}, {Recording, Playback}, {Playback, Ambient}, {Ambient, Kept}, {Kept, 0}}; !slices.Equal(got, want) {
		t.Fatalf("moves %v, want %v", got, want)
	}
}

// The last release does not deactivate the session on its caller's thread, and a hold of the same level before the deactivation runs keeps the session as it is, with no move at all.
func TestReleaseDefersDeactivation(t *testing.T) {
	moves := recordMoves(t, time.Hour, nil)
	release, _ := Hold(Playback)
	release()
	if level() != Playback {
		t.Fatalf("the session went to %v at the release, want it kept at Playback", level())
	}
	again, _ := Hold(Playback)
	holders.Lock()
	arranged := holders.deactivation != nil
	holders.Unlock()
	if arranged {
		t.Fatal("a hold left the deactivation arranged")
	}
	if got, want := moves(), []move{{0, Playback}}; !slices.Equal(got, want) {
		t.Fatalf("moves %v, want %v", got, want)
	}
	deactivateAfter = time.Millisecond
	again()
	waitFor(t, func() bool { return holders.applied == 0 })
	if got, want := moves(), []move{{0, Playback}, {Playback, 0}}; !slices.Equal(got, want) {
		t.Fatalf("moves %v, want %v", got, want)
	}
}

// A hold of another level before the deactivation runs moves the session from the level it still has.
func TestHoldBeforeDeactivationMovesAcross(t *testing.T) {
	moves := recordMoves(t, time.Hour, nil)
	release, _ := Hold(Playback)
	release()
	mixing, _ := Hold(Mixing)
	deactivateAfter = time.Millisecond
	mixing()
	waitFor(t, func() bool { return holders.applied == 0 })
	if got, want := moves(), []move{{0, Playback}, {Playback, Mixing}, {Mixing, 0}}; !slices.Equal(got, want) {
		t.Fatalf("moves %v, want %v", got, want)
	}
}

// A hold that comes while the deactivation runs waits for it, then activates the session again, so the two never overlap.
func TestHoldWaitsForDeactivation(t *testing.T) {
	started, finish := make(chan struct{}), make(chan struct{})
	var first sync.Once
	moves := recordMoves(t, time.Millisecond, func(m move) {
		if m.to == 0 {
			first.Do(func() {
				close(started)
				<-finish
			})
		}
	})
	release, _ := Hold(Ambient)
	release()
	<-started
	held := make(chan func())
	go func() {
		again, _ := Hold(Ambient)
		held <- again
	}()
	select {
	case <-held:
		t.Fatal("a hold went ahead of the deactivation running")
	case <-time.After(20 * time.Millisecond):
	}
	close(finish)
	again := <-held
	if level() != Ambient {
		t.Fatalf("the session is at %v after the hold, want Ambient", level())
	}
	again()
	waitFor(t, func() bool { return holders.applied == 0 })
	if got, want := moves(), []move{{0, Ambient}, {Ambient, 0}, {0, Ambient}, {Ambient, 0}}; !slices.Equal(got, want) {
		t.Fatalf("moves %v, want %v", got, want)
	}
}
