//go:build darwin && !ios

package apple

import (
	"slices"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The synthesizer loads, lists the voices installed and finds one by language, by a bare language and by ID, and names what it lacks.
func TestSpeakVoices(t *testing.T) {
	if !loadSpeak() {
		t.Fatal("AVSpeechSynthesizer does not load")
	}
	voices := speakVoices()
	i := slices.IndexFunc(voices, func(v driver.Voice) bool { return strings.HasPrefix(v.Lang, "en-") })
	if i < 0 {
		t.Fatalf("no English voice among %d", len(voices))
	}
	if voices[i].ID == "" || voices[i].Name == "" {
		t.Fatalf("voice %+v", voices[i])
	}
	for _, lang := range []string{voices[i].Lang, "en"} {
		if v, code := speakVoice(lang, ""); v == 0 || code != "" {
			t.Errorf("speakVoice(%q) = %v, %q", lang, v, code)
		}
	}
	if v, code := speakVoice("", voices[i].ID); v == 0 || GoString(v.Send(sel("identifier"))) != voices[i].ID || code != "" {
		t.Errorf("speakVoice of ID %q = %v, %q", voices[i].ID, v, code)
	}
	if _, code := speakVoice("", "dev.nsgo.no-such-voice"); code != driver.SpeakVoiceUnavailable {
		t.Errorf("an unknown ID gives %q", code)
	}
	if _, code := speakVoice("x-none", ""); code != driver.SpeakLanguageUnavailable {
		t.Errorf("an unknown language gives %q", code)
	}
}
