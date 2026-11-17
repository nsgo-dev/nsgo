//go:build js && wasm

package web

import (
	"encoding/binary"
	"syscall/js"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A real media element plays to its end, stays there through unrelated configuration, and replays only for a pause change or an explicit seek.
func TestFinishedVideoWaitsForReplay(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("needs a browser: run with nsgo test -target web")
	}
	wav := make([]byte, 44+9600) // 600ms of 8kHz mono PCM silence.
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 8000)
	binary.LittleEndian.PutUint32(wav[28:], 16000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(wav)-44))
	bytes := js.Global().Get("Uint8Array").New(len(wav))
	js.CopyBytesToJS(bytes, wav)
	blob := js.Global().Get("Blob").New([]any{bytes}, map[string]any{"type": "audio/wav"})
	url := js.Global().Get("URL").Call("createObjectURL", blob).String()
	defer js.Global().Get("URL").Call("revokeObjectURL", url)
	s := &surface{doc: doc}
	v := &view{el: doc.Call("createElement", "div")}
	s.views = []*view{v}
	doc.Get("body").Call("appendChild", v.el)
	s.createVideo(v)
	defer func() { v.video.release(); v.listeners.release(); v.el.Call("remove") }()
	ends, starts := 0, 0
	v.listeners.on(v.video.el, "ended", func(js.Value) { ends++ })
	v.listeners.on(v.video.el, "playing", func(js.Value) { starts++ })
	config := driver.VideoConfig{Source: url, Muted: true, Volume: 0.5, Rate: 1, ProgressInterval: time.Second}
	wait := func(want int) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); ends < want; time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("%d ends, %d starts, want %d ends (media error %v)", ends, starts, want, v.video.el.Get("error"))
			}
		}
		if ends != want || starts != want {
			t.Fatalf("%d ends, %d starts, want %d of each", ends, starts, want)
		}
	}
	s.setVideo(v, &config)
	wait(1)
	for _, change := range []func(){
		func() { config.Volume = 0.3 },
		func() { config.Muted = false },
		func() { config.Rate = 1.25 },
		func() { config.ResizeMode = driver.Cover },
		func() { config.Controls = true },
		func() { config.ProgressInterval = 50 * time.Millisecond },
		func() { config.Repeat = true },
	} {
		change()
		s.setVideo(v, &config)
		time.Sleep(100 * time.Millisecond)
		if starts != 1 || ends != 1 || !v.video.el.Get("paused").Bool() {
			t.Fatalf("unrelated config replayed the video: %d starts, %d ends", starts, ends)
		}
	}
	// Replays remain muted: an automated page has no user activation to grant audible playback, independently of the completion behavior.
	config.Muted, config.Repeat, config.Paused = true, false, true
	s.setVideo(v, &config)
	config.Paused = false
	s.setVideo(v, &config)
	wait(2)
	v.video.seek(200 * time.Millisecond)
	wait(3)
	config.Paused = true
	s.setVideo(v, &config)
	v.video.seek(200 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	if starts != 3 || ends != 3 {
		t.Fatalf("seek while paused replayed the video: %d starts, %d ends", starts, ends)
	}
	config.Paused = false
	s.setVideo(v, &config)
	wait(4)
	// Move the native scrubber without using Ref.Seek. Cover both a
	// settled seek and a play before the browser dispatches seeked.
	seeked := 0
	v.listeners.on(v.video.el, "seeked", func(js.Value) { seeked++ })
	for i, settle := range []bool{true, false} {
		config.Paused = true
		s.setVideo(v, &config)
		before := seeked
		v.video.el.Set("currentTime", 0.2)
		if settle {
			for deadline := time.Now().Add(5 * time.Second); seeked == before; time.Sleep(10 * time.Millisecond) {
				if time.Now().After(deadline) {
					t.Fatal("the native scrubber did not finish seeking")
				}
			}
		}
		config.Paused = false
		s.setVideo(v, &config)
		if got := v.video.el.Get("currentTime").Float(); got < 0.19 {
			t.Fatalf("play after native scrub (settled %v) rewound to %v", settle, got)
		}
		wait(5 + i)
	}
	other := js.Global().Get("URL").Call("createObjectURL", blob).String()
	defer js.Global().Get("URL").Call("revokeObjectURL", other)
	config.Source = other
	s.setVideo(v, &config)
	wait(7)
}
