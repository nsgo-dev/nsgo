//go:build ios

package apple

import (
	"log"

	"github.com/nsgo-dev/nsgo/internal/audiosession"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// videoSessions are the videos playing and the audio session each holds, which internal/audiosession weighs against the other holders of the shared AVAudioSession. Only the main thread touches it.
var videoSessions = map[*Video]videoSession{}

type videoSession struct {
	session driver.AudioSession
	release func()
}

// audioPlaying says video key plays with session, or, playing false, plays no more. AudioSessionDefault holds the session at the app's own category, so that it stays active under the playing video: deactivating it, as a holder of another level letting go would, stops the video.
func audioPlaying(key *Video, session driver.AudioSession, playing bool) {
	held, ok := videoSessions[key]
	if !playing {
		if ok {
			delete(videoSessions, key)
			held.release()
		}
		return
	}
	if ok && held.session == session {
		return
	}
	level := audiosession.Kept
	switch session {
	case driver.AudioSessionPlayback:
		level = audiosession.Playback
	case driver.AudioSessionMixWithOthers:
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
