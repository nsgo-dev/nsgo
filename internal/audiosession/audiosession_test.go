package audiosession

import "testing"

// The highest level held sets the session, and releasing the last goes back to none.
func TestHighestHolderWins(t *testing.T) {
	var applied []Level
	level := func() Level { return holders.applied }
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
	sound()
	applied = append(applied, level())
	want := []Level{Playback, Recording, Recording, Playback, Ambient, 0}
	for i := range want {
		if applied[i] != want[i] {
			t.Fatalf("applied %v, want %v", applied, want)
		}
	}
}
