// Package audiosession holds iOS's one AVAudioSession for every part of NSGo that sets it: videos, the audio package's players and sounds, speech and microphone. Each holder asks for a level, and the highest held sets the session; once none is held it goes back to the category from before the first and is deactivated, telling other apps they may play. It imports nothing of the module, so microphone and the backends both import it, and guards its holders with a mutex, so the main thread, audio workers and microphone's caller may all call it. Outside iOS it holds nothing.
package audiosession

import "sync"

// Level is what a holder needs of the session, from Ambient to Dictation; a higher one wins.
type Level uint8

const (
	// Ambient is every sound and a player with the default session: the Ambient category, which mixes with other apps' audio and which the Ring/Silent switch and the screen lock silence.
	Ambient Level = iota + 1
	// Mixing is a video or a player with AudioSessionMixWithOthers: Playback in the SpokenAudio mode, mixing with other apps' audio.
	Mixing
	// Playback is a video or a player with AudioSessionPlayback: Playback in the SpokenAudio mode, pausing other apps' audio.
	Playback
	// Recording is a recording, microphone.Capture: PlayAndRecord with AllowBluetoothHFP and DefaultToSpeaker in the Default mode, which the other levels' sounds play through and the Ring/Silent switch does not silence.
	Recording
	// Dictation is a speech session listening: Recording in the Measurement mode.
	Dictation
)

var holders struct {
	sync.Mutex
	// held counts the holders of each level.
	held [Dictation + 1]int
	// applied is the level the session is at, 0 for none.
	applied Level
}

// Hold sets the session to level, or keeps the higher one held, until release, which does nothing called again. When the session cannot be set and activated, Hold holds nothing and returns why.
func Hold(level Level) (release func(), err error) {
	holders.Lock()
	defer holders.Unlock()
	holders.held[level]++
	if err := settle(); err != nil {
		holders.held[level]--
		settle()
		return func() {}, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			holders.Lock()
			defer holders.Unlock()
			holders.held[level]--
			settle()
		})
	}, nil
}

// settle sets the session to the highest level held.
func settle() error {
	want := Level(0)
	for l := Dictation; l > 0; l-- {
		if holders.held[l] > 0 {
			want = l
			break
		}
	}
	if want == holders.applied {
		return nil
	}
	if err := apply(holders.applied, want); err != nil {
		return err
	}
	holders.applied = want
	return nil
}
