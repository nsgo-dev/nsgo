//go:build ios

package apple

import (
	"log"

	"github.com/nsgo-dev/nsgo/internal/audiosession"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// videoSessions are the videos that play with an audio session of their own, and how each lets it go, which internal/audiosession weighs against the other holders of the shared AVAudioSession. Only the main thread touches it.
var videoSessions = map[*Video]videoSession{}

type videoSession struct {
	session driver.AudioSession
	release func()
}

// audioPlaying says video key plays with session, or plays no more when session is AudioSessionDefault, which holds nothing.
func audioPlaying(key *Video, session driver.AudioSession) {
	held, ok := videoSessions[key]
	if ok && held.session == session {
		return
	}
	if session == driver.AudioSessionDefault {
		if ok {
			delete(videoSessions, key)
			held.release()
		}
		return
	}
	level := audiosession.Playback
	if session == driver.AudioSessionMixWithOthers {
		level = audiosession.Mixing
	}
	// The new level is held before the old one goes, so a playing video that changes its session moves the session across without deactivating it; when it cannot, the video keeps the old one.
	release, err := audiosession.Hold(level)
	if err != nil {
		log.Print("nsgo: cannot set and activate the audio session for a video")
		return
	}
	videoSessions[key] = videoSession{session, release}
	if ok {
		held.release()
	}
}
