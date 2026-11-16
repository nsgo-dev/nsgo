package driver

import "strings"

// SpeakCommand is what an ItemSpeak asks of the platform's speech synthesizer.
type SpeakCommand uint8

const (
	// SpeakStart speaks Text in the voice the request names, utterance Count, stopping the one speaking; the platform reports EventSpeak with Count once it finished, was stopped or failed.
	SpeakStart SpeakCommand = iota
	// SpeakStop stops utterance Count where it is, if it is the one speaking.
	SpeakStop
	// SpeakVoices asks for the voices installed; the platform answers EventSpeak with Count and the voices.
	SpeakVoices
)

var speakCommandNames = [...]string{"start", "stop", "voices"}

// String returns the command's name, as traces print it: start, stop, voices.
func (c SpeakCommand) String() string {
	if int(c) < len(speakCommandNames) {
		return speakCommandNames[c]
	}
	return "speak-command"
}

// SpeakRequest is an ItemSpeak's request.
type SpeakRequest struct {
	Command SpeakCommand
	// Text is what SpeakStart says.
	Text string
	// Lang is a BCP 47 tag, the app's locale when the app gives none, and Voice a voice's ID from SpeakVoices, "" for the platform's default voice of Lang. A Voice the platform does not have fails with SpeakVoiceUnavailable.
	Lang, Voice string
	// Rate and Pitch run from 0.5 to 2, 1 being the voice's own; core clamps them.
	Rate, Pitch float32
}

// The errors an utterance fails with, named as the Web Speech API's SpeechSynthesisErrorEvent's.
const (
	SpeakNotAllowed          = "not-allowed"
	SpeakLanguageUnavailable = "language-unavailable"
	SpeakVoiceUnavailable    = "voice-unavailable"
	SpeakTextTooLong         = "text-too-long"
	SpeakAudioBusy           = "audio-busy"
	SpeakUnavailable         = "synthesis-unavailable"
	SpeakFailed              = "synthesis-failed"
)

// SpeakEvent is an EventSpeak's report: an utterance's end, or the voices SpeakVoices asked for.
type SpeakEvent struct {
	// Stopped says the utterance was stopped before it finished, and Error, one of the Speak errors, why it failed; neither when it finished.
	Stopped bool
	Error   string
	// Voices are SpeakVoices' answer.
	Voices []Voice
}

// Voice is a voice installed on the platform.
type Voice struct {
	// ID names it in a SpeakRequest; Name is what the platform calls it, and Lang the BCP 47 tag of the language it speaks.
	ID, Name, Lang string
}

// SpeakScale maps a request's rate or pitch, 0.5 to 2 with 1 the voice's own and 0 standing for 1, onto a platform's scale: from at 0.5, mid at 1 and to at 2, linearly on each side of mid.
func SpeakScale(v, from, mid, to float32) float32 {
	switch {
	case v <= 0:
		return mid
	case v < 1:
		return mid - (1-v)*2*(mid-from)
	}
	return mid + (min(v, 2)-1)*(to-mid)
}

// LanguageTag is tag with '_' as '-', as BCP 47 writes it: Android's and Linux's locales use '_'.
func LanguageTag(tag string) string { return strings.ReplaceAll(tag, "_", "-") }
