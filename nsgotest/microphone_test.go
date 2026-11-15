package nsgotest

import (
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/microphone"
)

// TestMicrophoneInput checks that an app records a playable WAV from the fake, at the fake's rate, with levels, and that a test sees how long.
func TestMicrophoneInput(t *testing.T) {
	m := &Microphone{Rate: 16000}
	Start(t, func(cx *core.Scope) core.Node { return core.View{} }, MicrophoneInput(m))
	if !microphone.Supported() || !microphone.Allowed() {
		t.Fatal("the fake does not let the app record")
	}
	levels := make(chan microphone.Level, 10)
	r, err := microphone.RecordWAV("take.wav", microphone.RecordOptions{Rate: 44100, Level: func(l microphone.Level) { levels <- l }})
	if err != nil {
		t.Fatal(err)
	}
	if f := r.File(); f.Rate != 16000 {
		t.Errorf("rate = %d, want the fake's 16000", f.Rate)
	}
	m.Emit(make([]int16, 8000))
	m.Emit([]int16{16384, -16384})
	select {
	case l := <-levels:
		if l.Peak < 0 || l.Peak > 1 {
			t.Errorf("level %v", l)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no level arrived")
	}
	if _, err := microphone.RecordWAV("other.wav", microphone.RecordOptions{}); !errors.Is(err, microphone.ErrBusy) {
		t.Errorf("a second recording: %v, want ErrBusy", err)
	}
	r.Stop()
	got := m.Recordings()
	if len(got) != 1 || got[0].Duration != 8002*time.Second/16000 || got[0].Path != r.File().Path {
		t.Fatalf("Recordings = %+v, want the half second recorded at %s", got, r.File().Path)
	}
	data, err := os.ReadFile(got[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 44+2*8002 || string(data[:4]) != "RIFF" || binary.LittleEndian.Uint32(data[40:]) != 2*8002 || binary.LittleEndian.Uint32(data[24:]) != 16000 {
		t.Errorf("the file is no WAV of what was emitted: % x", data[:44])
	}
	if _, err := microphone.RecordWAV("take.wav", microphone.RecordOptions{}); !errors.Is(err, os.ErrExist) {
		t.Errorf("recording over a file: %v, want ErrExist", err)
	}
}

func TestMicrophoneInputInterrupts(t *testing.T) {
	m := &Microphone{}
	Start(t, func(cx *core.Scope) core.Node { return core.View{} }, MicrophoneInput(m))
	r, err := microphone.RecordWAV("ends.wav", microphone.RecordOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m.Interrupt(true)
	<-r.Ended()
	if !errors.Is(r.Err(), microphone.ErrInterrupted) {
		t.Errorf("Err = %v, want ErrInterrupted", r.Err())
	}
	r, err = microphone.RecordWAV("pauses.wav", microphone.RecordOptions{PauseOnInterruption: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Interrupt(true)
	m.Emit(make([]int16, 480))
	if !r.Interrupted() {
		t.Error("not interrupted while silenced")
	}
	m.Interrupt(false)
	m.Emit(make([]int16, 480))
	// A recording without levels still tells how long it ran.
	if d := r.File().Duration; d != 10*time.Millisecond {
		t.Errorf("Duration while recording = %v, want 10ms", d)
	}
	r.Stop()
	if d := r.File().Duration; d != 10*time.Millisecond {
		t.Errorf("Duration = %v, want only what was heard outside the silence", d)
	}
}

func TestMicrophoneInputDenies(t *testing.T) {
	Start(t, func(cx *core.Scope) core.Node { return core.View{} }, MicrophoneInput(&Microphone{Denied: true}))
	if err := microphone.Authorize(); !errors.Is(err, microphone.ErrDenied) {
		t.Errorf("Authorize = %v, want ErrDenied", err)
	}
	if _, err := microphone.RecordWAV("take.wav", microphone.RecordOptions{}); !errors.Is(err, microphone.ErrDenied) {
		t.Errorf("RecordWAV = %v, want ErrDenied", err)
	}
	if microphone.Allowed() {
		t.Error("Allowed while denied")
	}
}

// TestMicrophoneInputIsOneAtATime checks that a second app's fake fails its test rather than taking the first one's recordings.
func TestMicrophoneInputIsOneAtATime(t *testing.T) {
	root := func(cx *core.Scope) core.Node { return core.View{} }
	Start(t, root, MicrophoneInput(&Microphone{}))
	msg := fails(t, func(tb testing.TB) { Start(tb, root, MicrophoneInput(&Microphone{})) })
	if !strings.Contains(msg, "cannot run in parallel") {
		t.Errorf("a second fake: %q", msg)
	}
}
