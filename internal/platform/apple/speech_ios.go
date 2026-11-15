//go:build ios

package apple

import (
	"errors"

	"github.com/nsgo-dev/nsgo/internal/audiosession"
)

// holdSpeechSession holds the shared AVAudioSession at the level that records for dictation as long as the session runs, which internal/audiosession weighs against its other holders.
func holdSpeechSession() (leave func(), err error) {
	leave, err = audiosession.Hold(audiosession.Dictation)
	if err != nil {
		return nil, errors.New("apple: cannot activate the audio session for speech")
	}
	return leave, nil
}
