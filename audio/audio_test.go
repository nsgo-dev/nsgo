package audio_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/audio"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

var playsAudio = nsgotest.Environment(core.Environment{Audio: true})

// A tap's sound is loaded ahead and played in the batch of the tap's turn.
func TestASoundPlaysOnATap(t *testing.T) {
	var click *audio.Sound
	var loadErr error
	var app *nsgotest.App
	app = nsgotest.Start(t, func(cx *core.Scope) core.Node {
		audio.LoadSound(cx, audio.Source{URI: "asset:sounds/click.wav"}, func(s *audio.Sound, err error) { click, loadErr = s, err })
		audio.LoadSound(cx, audio.Source{URI: "asset:sounds/long.wav"}, func(_ *audio.Sound, err error) {
			if !errors.Is(err, audio.QuotaExceeded) {
				t.Errorf("a sound of a minute: %v", err)
			}
		})
		audio.LoadSound(cx, audio.Source{URI: "asset:sounds/missing.flac"}, func(_ *audio.Sound, err error) {
			if !errors.Is(err, audio.Decode) {
				t.Errorf("a sound the platform cannot read: %v", err)
			}
		})
		return core.View{TestID: "pad", Class: "size-12", OnPress: func(core.PressEvent) {
			click.Play(audio.Voice{})
			click.Play(audio.Voice{Volume: 0.5, Rate: 1.2})
		}}
	}, playsAudio, func(a *nsgotest.App) {
		a.AudioDurations = map[string]time.Duration{"asset:sounds/click.wav": 80 * time.Millisecond, "asset:sounds/long.wav": time.Minute}
	})
	if loadErr != nil || click == nil || click.Duration() != 80*time.Millisecond {
		t.Fatalf("click %v, %v", click, loadErr)
	}
	app.Press("id:pad")
	if len(app.Sounds) != 2 {
		t.Fatalf("%d sounds decoded", len(app.Sounds))
	}
	s := app.Sounds[0]
	if s.Source.URI != "asset:sounds/click.wav" || !slices.Equal(s.Plays, []audio.Voice{{Volume: 1, Rate: 1}, {Volume: 0.5, Rate: 1.2}}) {
		t.Fatalf("plays %+v", s)
	}
	if !app.Sounds[1].Unloaded {
		t.Fatal("the cut sound stays loaded")
	}
}

func TestAPlayerPlaysOnTheClock(t *testing.T) {
	var player *audio.Player
	var log []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		player = audio.NewPlayer(cx, audio.Source{URI: "https://example.com/voice.m4a"}, audio.PlayerOptions{
			OnLoad:     func(d time.Duration) { log = append(log, fmt.Sprint("load ", d)) },
			OnProgress: func(p, _ time.Duration) { log = append(log, fmt.Sprint(p.Round(time.Millisecond))) },
			OnEnd:      func() { log = append(log, "end") },
		})
		return core.View{}
	}, playsAudio, func(a *nsgotest.App) {
		a.AudioDurations = map[string]time.Duration{"https://example.com/voice.m4a": time.Second}
	})
	if len(app.Players) != 0 {
		t.Fatal("a new player opened on the platform")
	}
	var played []error
	player.Play(func(err error) { played = append(played, err) })
	app.Settle()
	if len(played) != 1 || played[0] != nil || player.State() != audio.PlayerPlaying {
		t.Fatalf("played %v, state %v", played, player.State())
	}
	app.Advance(500 * time.Millisecond)
	if player.Position() != 500*time.Millisecond {
		t.Fatalf("position %v", player.Position())
	}
	player.Pause()
	app.Settle()
	player.Seek(800 * time.Millisecond)
	app.Settle()
	app.Advance(time.Second)
	if player.Position() != 800*time.Millisecond || player.State() != audio.PlayerPaused {
		t.Fatalf("paused at %v, %v", player.Position(), player.State())
	}
	player.Play(nil)
	app.Advance(time.Second)
	want := []string{"load 1s", "250ms", "500ms", "500ms", "800ms", "1s", "end"}
	if !slices.Equal(log, want) {
		t.Fatalf("log %q, want %q", log, want)
	}
	p := app.Players[0]
	if want := []string{"open", "resume", "pause", "seek", "resume"}; !slices.Equal(p.Commands, want) || player.State() != audio.PlayerEnded {
		t.Fatalf("commands %q, state %v", p.Commands, player.State())
	}

	// Looping, it plays on past its end without ending, its position starting over.
	log = nil
	player.SetLoop(true)
	player.Play(nil)
	app.Advance(1100 * time.Millisecond)
	if want := []string{"250ms", "500ms", "750ms", "0s"}; player.State() != audio.PlayerPlaying || !slices.Equal(log, want) {
		t.Fatalf("looping: %v, %q, want %q", player.State(), log, want)
	}
	player.Close()
	app.Settle()
	if !p.Closed || p.Playing {
		t.Fatalf("closed player %+v", p)
	}
}

func TestAPlayerTheBrowserRefuses(t *testing.T) {
	var player *audio.Player
	var errs []error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		player = audio.NewPlayer(cx, audio.Source{URI: "asset:a.mp3"}, audio.PlayerOptions{OnError: func(err error) { errs = append(errs, err) }})
		return core.View{}
	}, playsAudio, func(a *nsgotest.App) {
		a.AudioDurations = map[string]time.Duration{"asset:a.mp3": time.Minute}
		a.RefusePlayback = true
	})
	player.Play(func(err error) { errs = append(errs, err) })
	app.Settle()
	if len(errs) != 2 || !errors.Is(errs[0], audio.NotAllowed) || !errors.Is(errs[1], audio.NotAllowed) || player.State() != audio.PlayerFailed {
		t.Fatalf("errors %v, state %v", errs, player.State())
	}
	app.RefusePlayback = false
	player.Play(nil)
	app.Settle()
	if player.State() != audio.PlayerPlaying {
		t.Fatalf("after a gesture: %v", player.State())
	}
}

func TestAudioUnavailable(t *testing.T) {
	var err error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		audio.LoadSound(cx, audio.Source{URI: "asset:a.wav"}, func(_ *audio.Sound, e error) { err = e })
		return core.View{}
	})
	if !errors.Is(err, audio.Unavailable) || len(app.Sounds) != 0 {
		t.Fatalf("err %v, %d sounds", err, len(app.Sounds))
	}
}

func TestSoundPlaybackControlsAndEnds(t *testing.T) {
	const uri = "asset:effect.wav"
	var sound *audio.Sound
	var changed *core.Signal[*audio.Playback]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		changed = core.NewSignal[*audio.Playback](cx, nil)
		audio.LoadSound(cx, audio.Source{URI: uri}, func(s *audio.Sound, err error) {
			if err != nil {
				t.Error(err)
			}
			sound = s
		})
		return core.Text{TestID: "status", ContentFunc: func() string {
			if changed.Get().Ended() {
				return "Ended"
			}
			return "Playing"
		}}
	}, playsAudio, func(a *nsgotest.App) { a.AudioDurations = map[string]time.Duration{uri: time.Second} })
	first := sound.Play(audio.Voice{})
	second := sound.Play(audio.Voice{Muted: true})
	changed.Set(second)
	app.Settle()
	if first.Ended() || second.Ended() || app.Find("id:status").Text != "Playing" {
		t.Fatal("two independent playbacks should be active")
	}
	p := app.Sounds[0].Playbacks
	if p[1].Volume != 0 {
		t.Fatalf("the muted playback started with volume %v", p[1].Volume)
	}
	first.Stop()
	second.SetVolume(0.25)
	second.SetRate(2)
	app.Settle()
	if !first.Ended() || second.Ended() || !p[0].Ended || p[1].Ended || p[1].Volume != 0.25 || p[1].Rate != 2 {
		t.Fatalf("independent stop and settings gave %+v, %+v", p[0], p[1])
	}
	second.SetVolume(0)
	app.Settle()
	if p[1].Volume != 0 || second.Ended() {
		t.Fatal("zero volume must silence a playback without ending it")
	}
	app.Advance(499 * time.Millisecond)
	if second.Ended() {
		t.Fatal("playback ended before the source duration at its new rate")
	}
	app.Advance(2 * time.Millisecond)
	if !second.Ended() || !p[1].Ended || app.Find("id:status").Text != "Ended" {
		t.Fatal("the reactive playback state did not observe completion")
	}

	plays := make([]*audio.Playback, 33)
	for i := range plays {
		plays[i] = sound.Play(audio.Voice{})
	}
	app.Settle()
	for i, play := range plays {
		if play.Ended() != (i == 0) {
			t.Fatalf("playback %d: ended = %v", i, play.Ended())
		}
	}
	sound.Stop()
	app.Settle()
	if !plays[len(plays)-1].Ended() {
		t.Fatal("Sound.Stop left a playback active")
	}
	last := sound.Play(audio.Voice{})
	app.Settle()
	sound.Close()
	app.Settle()
	if !last.Ended() || !sound.Play(audio.Voice{}).Ended() {
		t.Fatal("closing the buffer left an active playback")
	}
}

func TestPlayerMuteKeepsItsTimelineAndVolume(t *testing.T) {
	var player *audio.Player
	const uri = "asset:music.wav"
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		player = audio.NewPlayer(cx, audio.Source{URI: uri}, audio.PlayerOptions{})
		return core.View{}
	}, playsAudio, func(a *nsgotest.App) { a.AudioDurations = map[string]time.Duration{uri: time.Second} })

	player.SetMuted(true)
	player.Play(nil)
	app.Settle()
	app.Advance(250 * time.Millisecond)
	model := app.Players[0]
	if !model.Playing || model.Volume != 0 || player.Position() != 250*time.Millisecond {
		t.Fatalf("muted play = %+v, position = %v", model, player.Position())
	}
	player.SetVolume(0.5)
	app.Settle()
	if model.Volume != 0 {
		t.Fatalf("setting volume removed mute: %+v", model)
	}
	player.SetMuted(false)
	app.Advance(250 * time.Millisecond)
	if !model.Playing || model.Volume != 0.5 || player.Position() != 500*time.Millisecond {
		t.Fatalf("unmuted play = %+v, position = %v", model, player.Position())
	}
	if want := []string{"open", "resume", "set", "set"}; !slices.Equal(model.Commands, want) {
		t.Fatalf("commands = %v, want %v", model.Commands, want)
	}
}
