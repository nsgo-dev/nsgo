//go:build js

package web

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Run in a browser, which has speechSynthesis: nsgo test -target web. A second utterance stops the first and a stop the second, each reported once, or each is refused, a voice the browser lacks fails, and voices are answered, though a headless browser may have none.
func TestSpeak(t *testing.T) {
	if !speakAvailable() {
		t.Skip("no speechSynthesis here: run with nsgo test -target web")
	}
	events := make(chan driver.Event, 16)
	dispatch := func(ev driver.Event) { events <- ev }
	next := func() (uint32, driver.SpeakEvent) {
		select {
		case ev := <-events:
			return ev.Count, *ev.Data.(*driver.SpeakEvent)
		case <-time.After(5 * time.Second):
			t.Fatal("no report")
		}
		return 0, driver.SpeakEvent{}
	}
	speak(1, driver.SpeakRequest{Command: driver.SpeakVoices}, dispatch)
	if id, e := next(); id != 1 || e.Error != "" {
		t.Fatalf("voices answered %d %+v", id, e)
	}
	speak(2, driver.SpeakRequest{Command: driver.SpeakStart, Text: "The first sentence, which the second cuts.", Lang: "en-US", Rate: 1, Pitch: 1}, dispatch)
	speak(3, driver.SpeakRequest{Command: driver.SpeakStart, Text: "The second sentence.", Lang: "en-US", Rate: 1, Pitch: 1}, dispatch)
	// Chrome refuses to speak before the page's first user activation, as a test's page has none, and reports it at once.
	if id, e := next(); id != 2 || !e.Stopped && e.Error != driver.SpeakNotAllowed {
		t.Fatalf("the first ended %d %+v", id, e)
	}
	speak(3, driver.SpeakRequest{Command: driver.SpeakStop}, dispatch)
	if id, e := next(); id != 3 || !e.Stopped && e.Error != driver.SpeakNotAllowed {
		t.Fatalf("the second ended %d %+v", id, e)
	}
	speak(4, driver.SpeakRequest{Command: driver.SpeakStart, Text: "x", Voice: "dev.nsgo.no-such-voice"}, dispatch)
	if id, e := next(); id != 4 || e.Error != driver.SpeakVoiceUnavailable {
		t.Fatalf("an unknown voice ended %d %+v", id, e)
	}
	select {
	case ev := <-events:
		t.Fatalf("a late report %+v", ev)
	case <-time.After(300 * time.Millisecond):
	}
	for code, want := range map[string]driver.SpeakEvent{"canceled": {Stopped: true}, "interrupted": {Stopped: true},
		"text-too-long": {Error: driver.SpeakTextTooLong}, "audio-hardware": {Error: driver.SpeakFailed}} {
		if got := webSpeakEnd(code); got.Stopped != want.Stopped || got.Error != want.Error {
			t.Errorf("webSpeakEnd(%q) = %+v", code, got)
		}
	}
}
