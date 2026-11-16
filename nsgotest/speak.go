package nsgotest

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Utterance is text the app spoke with Scope.Speak.
type Utterance struct {
	// Text is what it said, Lang its language, Voice the voice's ID, "" for the default one, and Rate and Pitch how it spoke, from 0.5 to 2.
	Text, Lang, Voice string
	Rate, Pitch       float32
	// Stopped says the app stopped it, or spoke another, before it finished, and Error why FinishSpeaking failed it.
	Stopped bool
	Error   core.SpeakError
	// Finished says it ended, however it did.
	Finished bool
	id       uint32
}

// speak answers an ItemSpeak as a synthesizer would: an utterance speaks until FinishSpeaking ends it, or the app stops it or speaks another, and SpeakVoices answers with Voices.
func (a *App) speak(id uint32, r driver.SpeakRequest) {
	switch r.Command {
	case driver.SpeakStart:
		if u := a.Speaking(); u != nil {
			u.Stopped, u.Finished = true, true
			a.post(driver.Event{Kind: driver.EventSpeak, Count: u.id, Data: &driver.SpeakEvent{Stopped: true}})
		}
		a.Spoken = append(a.Spoken, &Utterance{Text: r.Text, Lang: r.Lang, Voice: r.Voice, Rate: r.Rate, Pitch: r.Pitch, id: id})
	case driver.SpeakStop:
		if u := a.Speaking(); u != nil && u.id == id {
			u.Stopped, u.Finished = true, true
			a.post(driver.Event{Kind: driver.EventSpeak, Count: id, Data: &driver.SpeakEvent{Stopped: true}})
		}
	case driver.SpeakVoices:
		a.post(driver.Event{Kind: driver.EventSpeak, Count: id, Data: &driver.SpeakEvent{Voices: append([]core.VoiceInfo(nil), a.Voices...)}})
	}
}

// Speaking is the utterance being spoken, nil when none is.
func (a *App) Speaking() *Utterance {
	if n := len(a.Spoken); n > 0 && !a.Spoken[n-1].Finished {
		return a.Spoken[n-1]
	}
	return nil
}

// FinishSpeaking ends the utterance being spoken, as it finished when err is "", or failed with err, and settles.
func (a *App) FinishSpeaking(err core.SpeakError) {
	a.t.Helper()
	u := a.Speaking()
	if u == nil {
		a.t.Fatal("nsgotest: nothing is being spoken")
	}
	u.Finished, u.Error = true, err
	a.post(driver.Event{Kind: driver.EventSpeak, Count: u.id, Data: &driver.SpeakEvent{Error: string(err)}})
	a.Settle()
}
