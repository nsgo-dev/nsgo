//go:build windows

package windows

import (
	"log"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// menuUtterance numbers what the Speech menu's Start Speaking says. Core numbers its utterances from 1, so the synthesizer's run of 0 is the menu's, which reports its end to no engine.
const menuUtterance = 0

// speakMenu carries out Start Speaking, which reads the focused field's selection, or its whole text without one, in a voice for the app's language or else Windows' default one, stopping whatever speaks, and Stop Speaking, which stops what Start Speaking said, from any window.
func (w *window) speakMenu(start bool) {
	if !start {
		if run := speaking.run; run != nil && run.id == menuUtterance {
			stopSpeaking()
		}
		return
	}
	text := w.focusedField().speechText()
	if strings.TrimSpace(text) == "" {
		return
	}
	stopSpeaking()
	// A voice for the app's language, else Windows' default voice, which reads the text whatever its language, as macOS's does.
	code := w.startSpeaking(menuUtterance, driver.SpeakRequest{Command: driver.SpeakStart, Lang: w.env.Locale, Text: text})
	if code == driver.SpeakLanguageUnavailable {
		code = w.startSpeaking(menuUtterance, driver.SpeakRequest{Command: driver.SpeakStart, Text: text})
	}
	if code != "" {
		log.Printf("nsgo: Start Speaking: %s", code)
	}
}

// speechText is what Start Speaking reads of the field: its selection, or its whole text without one, and nothing of a password field or of none.
func (in *textInput) speechText() string {
	if in == nil || in.masked() {
		return ""
	}
	if in.model.hasSelection() {
		return in.model.selected()
	}
	return in.model.text
}
