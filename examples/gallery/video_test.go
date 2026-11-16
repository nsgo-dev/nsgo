package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Run on native targets: configuration changes must leave a finished player alone, while an explicit replay or seek can play it again.
func TestFinishedVideoWaitsForReplay(t *testing.T) {
	switch nsgotest.Target() {
	case "", "web", "watchos_simulator":
		t.Skip("needs a native video player and a writable filesystem")
	}
	const duration = 600 * time.Millisecond
	wav := make([]byte, 44+9600)
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
	source := filepath.Join(t.TempDir(), "silence.wav")
	if err := os.WriteFile(source, wav, 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(filepath.Dir(source), "other.wav")
	if err := os.WriteFile(other, wav, 0o600); err != nil {
		t.Fatal(err)
	}
	var ends, progress atomic.Int32
	var minProgress atomic.Int64
	var recording atomic.Bool
	var failure atomic.Value
	failure.Store("")
	var volume func()
	var mute func()
	var rate func()
	var fit func()
	var pause func(bool)
	var replaceSource func()
	var ref core.Ref
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		src := core.NewSignal(cx, source)
		paused := core.NewSignal(cx, false)
		vol := core.NewSignal(cx, float32(0.5))
		muted := core.NewSignal(cx, false)
		speed := core.NewSignal(cx, float32(1))
		resize := core.NewSignal(cx, core.Contain)
		volume = func() { vol.Set(0.3) }
		mute = func() { muted.Set(true) }
		rate = func() { speed.Set(1.25) }
		fit = func() { resize.Set(core.Cover) }
		pause = paused.Set
		replaceSource = func() { src.Set(other) }
		return core.Video{SourceFunc: src.Get, Ref: &ref, PausedFunc: paused.Get,
			VolumeFunc: vol.Get, MutedFunc: muted.Get, RateFunc: speed.Get, ResizeModeFunc: resize.Get,
			Style: core.Style{Width: core.Pt(160), Height: core.Pt(90)}, ProgressInterval: 30 * time.Millisecond,
			OnEnd: func() { ends.Add(1) },
			OnProgress: func(position, _ time.Duration) {
				progress.Add(1)
				if recording.Load() {
					for old := minProgress.Load(); int64(position) < old; old = minProgress.Load() {
						if minProgress.CompareAndSwap(old, int64(position)) {
							break
						}
					}
				}
			},
			OnError: func(err error) { failure.Store(err.Error()) }}
	})
	waitEnd := func(want int32) {
		t.Helper()
		app.WaitUntil(10*time.Second, func() bool { return ends.Load() >= want || failure.Load().(string) != "" })
		if got := failure.Load().(string); got != "" {
			t.Fatal(got)
		}
		if got := ends.Load(); got != want {
			t.Fatalf("%d ends, want %d", got, want)
		}
	}
	waitEnd(1)
	quiet := func() {
		t.Helper()
		beforeEnds, beforeProgress := ends.Load(), progress.Load()
		app.Settle()
		time.Sleep(duration / 3)
		app.Settle()
		if ends.Load() != beforeEnds || progress.Load() != beforeProgress {
			t.Fatalf("a finished video reported new events: ends %d -> %d, progress %d -> %d",
				beforeEnds, ends.Load(), beforeProgress, progress.Load())
		}
		if got := failure.Load().(string); got != "" {
			t.Fatal(got)
		}
	}
	for _, change := range []struct {
		name string
		do   func()
	}{{"volume", volume}, {"muted", mute}, {"rate", rate}, {"resize", fit}} {
		t.Log(change.name)
		scope.Post(change.do)
		quiet()
	}
	scope.Post(func() { pause(true) })
	app.Settle()
	scope.Post(func() { pause(false) })
	waitEnd(2)
	minProgress.Store(int64(duration))
	recording.Store(true)
	scope.Post(func() { ref.Seek(200 * time.Millisecond) })
	waitEnd(3)
	recording.Store(false)
	if got := time.Duration(minProgress.Load()); got < 150*time.Millisecond || got >= duration {
		t.Fatalf("seek replay's first progress = %v, want between 150ms and %v", got, duration)
	}
	scope.Post(func() { pause(true) })
	app.Settle()
	scope.Post(func() { ref.Seek(200 * time.Millisecond) })
	// A paused seek may report its new position once. It must not keep producing progress or reach another end while paused.
	app.Settle()
	quiet()
	scope.Post(func() { pause(false) })
	waitEnd(4)
	scope.Post(replaceSource)
	waitEnd(5)
}
