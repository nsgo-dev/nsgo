//go:build darwin && !ios

package macos

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// silence writes a WAV file of d of silence, which AVPlayer plays as it does a video's sound.
func silence(t *testing.T, d time.Duration) string {
	const rate = 8000
	samples := int(d.Seconds() * rate)
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
	path := filepath.Join(t.TempDir(), "silence.wav")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// videoBackend is the backend of the app's one video, which shows without controls.
func videoBackend(t *testing.T) *apple.Video {
	t.Helper()
	for _, v := range views {
		if v.video != nil && v.video.View() != 0 {
			return v.video
		}
	}
	t.Error("no video shows")
	return nil
}

func videoPlayer(t *testing.T) objc.ID {
	t.Helper()
	if v := videoBackend(t); v != nil {
		return v.View().Send(sel("layer")).Send(sel("player"))
	}
	return 0
}

// TestVideoReportsWhatItsPlayerDidByItself pauses and plays a playing video's AVPlayer as its controls do: each is reported once, the pause holds through a config that changes only the volume, the controls play at the app's Rate, the app's own pause and play are not reported, and the end is EventEnd's alone. Run ends a process's app for good, so the test runs in a process of its own.
func TestVideoReportsWhatItsPlayerDidByItself(t *testing.T) {
	if os.Getenv("NSGO_TEST_VIDEO_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_VIDEO_PROCESS="+t.Name(), "HOME="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	source := silence(t, 2*time.Second)
	var mu sync.Mutex
	var heard []string
	// replayed is the rate the controls' play gave the player.
	var replayed float32
	hear := func(s string) {
		mu.Lock()
		heard = append(heard, s)
		mu.Unlock()
	}
	// later runs fn on the main thread after d, as the user would act.
	later := func(d time.Duration, fn func()) { time.AfterFunc(d, func() { apple.OnMain(fn) }) }
	app := func(cx *core.Scope) core.Node {
		paused, volume := core.NewSignal(cx, false), core.NewSignal(cx, float32(0.5))
		pauses := 0
		return core.Video{Source: source, PausedFunc: paused.Get, VolumeFunc: volume.Get, Rate: 1.5,
			Style: core.Style{Width: core.Pt(160), Height: core.Pt(90)},
			OnLoad: func(core.VideoInfo) {
				hear("load")
				later(300*time.Millisecond, func() { videoPlayer(t).Send(sel("pause")) })
			},
			OnPause: func(r core.VideoPauseReason) {
				hear("pause " + r.String())
				if pauses++; pauses > 1 {
					return
				}
				cx.Post(func() { volume.Set(0.4) })
				later(300*time.Millisecond, func() {
					if objc.Send[float32](videoPlayer(t), sel("rate")) == 0 {
						hear("held")
					}
					v := videoBackend(t)
					if v == nil {
						return
					}
					// The engine can react to OnPlay immediately. Reporting
					// from inside play lets its pause reenter AVPlayer before
					// play finished, which macOS 15 follows with another play.
					dispatch := v.Dispatch
					v.Dispatch = func(ev driver.Event) {
						if ev.Kind == driver.EventVideoPlay {
							t.Error("reported play before the native play operation returned")
						}
						dispatch(ev)
					}
					videoPlayer(t).Send(sel("play"))
					v.Dispatch = dispatch
					mu.Lock()
					replayed = objc.Send[float32](videoPlayer(t), sel("rate"))
					mu.Unlock()
				})
			},
			OnPlay: func() {
				hear("play")
				// The app's own pause and play are not reported.
				cx.Post(func() { paused.Set(true) })
				later(200*time.Millisecond, func() { cx.Post(func() { paused.Set(false) }) })
			},
			OnEnd: func() {
				hear("end")
				later(500*time.Millisecond, func() { cx.Post(cx.Quit) })
			},
			OnError: func(err error) { t.Error(err); cx.Post(cx.Quit) },
		}
	}
	watchdog := time.AfterFunc(30*time.Second, func() {
		mu.Lock()
		defer mu.Unlock()
		panic("the video did not play to its end, having reported " + strings.Join(heard, ", "))
	})
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Video", Width: 200, Height: 120}) })
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"load", "pause controls", "held", "play", "end"}; !slices.Equal(heard, want) {
		t.Errorf("heard %q, want %q", heard, want)
	}
	if replayed != 1.5 {
		t.Errorf("the controls played at %v, want the app's 1.5", replayed)
	}
}
