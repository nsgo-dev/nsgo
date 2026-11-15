//go:build darwin && !ios

package macos

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// silentWAV is length of silence as a WAV: 16-bit mono at 8 kHz.
func silentWAV(length time.Duration) []byte {
	const rate = 8000
	samples := int(length.Seconds() * rate)
	b := make([]byte, 44+2*samples)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+2*samples))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1) // PCM
	binary.LittleEndian.PutUint16(b[22:], 1) // mono
	binary.LittleEndian.PutUint32(b[24:], rate)
	binary.LittleEndian.PutUint32(b[28:], 2*rate)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(2*samples))
	return b
}

// TestAudioPlaysThroughTheSurface loads a sound and plays it, then plays a player to its end, through a window's engine, as an app does: the core's commands reach the surface's audio and its reports come back as the core's callbacks. A process runs one app, as Run registers its classes once, so the test runs in a process of its own.
func TestAudioPlaysThroughTheSurface(t *testing.T) {
	if os.Getenv("NSGO_TEST_AUDIO_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_AUDIO_PROCESS="+t.Name(), "HOME="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	got := make(chan string, 16)
	app := func(cx *core.Scope) core.Node {
		if !core.Untracked(cx, cx.Env().CanPlayAudio) {
			got <- "no audio"
			cx.Quit()
			return core.View{}
		}
		cx.LoadSound(core.AudioSource{Data: silentWAV(100 * time.Millisecond)}, func(s *core.Sound, err error) {
			if err != nil {
				got <- "sound: " + err.Error()
				cx.Quit()
				return
			}
			got <- fmt.Sprintf("sound of %v", s.Duration())
			s.Play(core.SoundVoice{Volume: 0.01})
			var p *core.AudioPlayer
			p = cx.NewAudioPlayer(core.AudioSource{Data: silentWAV(300 * time.Millisecond)}, core.AudioPlayerOptions{
				OnEnd: func() {
					got <- fmt.Sprintf("%v at %v of %v", p.State(), p.Position(), p.Duration())
					p.Close()
					cx.Quit()
				},
				OnError: func(err error) {
					got <- "player: " + err.Error()
					cx.Quit()
				},
			})
			p.SetVolume(0.01)
			p.Play(func(err error) {
				if err == nil {
					got <- "playing"
				}
			})
		})
		return core.View{}
	}
	watchdog := time.AfterFunc(20*time.Second, func() { panic("the app did not quit after its player ended") })
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Audio", Width: 200, Height: 100}) })
	close(got)
	var lines []string
	for line := range got {
		lines = append(lines, line)
	}
	want := []string{"sound of 100ms", "playing", "ended at 300ms of 300ms"}
	if fmt.Sprint(lines) != fmt.Sprint(want) {
		t.Errorf("the app saw %q, want %q", lines, want)
	}
}
