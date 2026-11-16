//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Speaking through the page's speechSynthesis, which speaks one utterance at a time for the page: a new one cancels the one before.

// speaking is the utterance the page speaks, which this keeps from the garbage collector until it ends, as Chrome drops the events of one nothing holds.
var speaking struct {
	id        uint32
	utterance js.Value
	end, fail js.Func
	dispatch  func(driver.Event)
}

// speakAvailable is Environment.Speak: the browser has speechSynthesis.
func speakAvailable() bool {
	g := js.Global()
	return g.Get("speechSynthesis").Truthy() && g.Get("SpeechSynthesisUtterance").Truthy()
}

// voicesWait is how long a request for voices waits, in milliseconds, for a browser that loads them after the page, as Chrome does, to say it has.
const voicesWait = 1500

// speak carries out an ItemSpeak of utterance or request id, reporting EventSpeak through dispatch.
func speak(id uint32, r driver.SpeakRequest, dispatch func(driver.Event)) {
	synth := js.Global().Get("speechSynthesis")
	report := func(e driver.SpeakEvent) { dispatch(driver.Event{Kind: driver.EventSpeak, Count: id, Data: &e}) }
	if !synth.Truthy() {
		if r.Command != driver.SpeakStop {
			report(driver.SpeakEvent{Error: driver.SpeakUnavailable})
		}
		return
	}
	switch r.Command {
	case driver.SpeakStart:
		stopSpeaking(synth)
		u := js.Global().Get("SpeechSynthesisUtterance").New(r.Text)
		u.Set("lang", r.Lang)
		u.Set("rate", driver.SpeakScale(r.Rate, 0.5, 1, 2))
		u.Set("pitch", driver.SpeakScale(r.Pitch, 0.5, 1, 2))
		if voice, code := webVoice(synth, r.Lang, r.Voice); code != "" {
			report(driver.SpeakEvent{Error: code})
			return
		} else if voice.Truthy() {
			u.Set("voice", voice)
		}
		ended := func(e driver.SpeakEvent) {
			if speaking.id != id || !speaking.utterance.Equal(u) {
				return
			}
			releaseSpeaking()
			report(e)
		}
		speaking.id, speaking.utterance, speaking.dispatch = id, u, dispatch
		speaking.end = js.FuncOf(func(js.Value, []js.Value) any {
			ended(driver.SpeakEvent{})
			return nil
		})
		speaking.fail = js.FuncOf(func(_ js.Value, args []js.Value) any {
			code := ""
			if len(args) > 0 {
				code = args[0].Get("error").String()
			}
			ended(webSpeakEnd(code))
			return nil
		})
		u.Set("onend", speaking.end)
		u.Set("onerror", speaking.fail)
		synth.Call("speak", u)
	case driver.SpeakStop:
		if speaking.id == id && speaking.utterance.Truthy() {
			stopSpeaking(synth)
		}
	case driver.SpeakVoices:
		if voices := synth.Call("getVoices"); voices.Length() > 0 {
			report(driver.SpeakEvent{Voices: webVoices(voices)})
			return
		}
		// The browser may not have loaded them yet: wait for it to say so, or for voicesWait.
		var changed, timeout js.Func
		var timer js.Value
		answered := false
		answer := func(js.Value, []js.Value) any {
			if answered {
				return nil
			}
			answered = true
			synth.Call("removeEventListener", "voiceschanged", changed)
			js.Global().Call("clearTimeout", timer)
			changed.Release()
			timeout.Release()
			report(driver.SpeakEvent{Voices: webVoices(synth.Call("getVoices"))})
			return nil
		}
		changed, timeout = js.FuncOf(answer), js.FuncOf(answer)
		synth.Call("addEventListener", "voiceschanged", changed)
		timer = js.Global().Call("setTimeout", timeout, voicesWait)
	}
}

// stopSpeaking cancels the utterance the page speaks, which reports itself stopped.
func stopSpeaking(synth js.Value) {
	if !speaking.utterance.Truthy() {
		return
	}
	id, dispatch := speaking.id, speaking.dispatch
	releaseSpeaking()
	synth.Call("cancel")
	dispatch(driver.Event{Kind: driver.EventSpeak, Count: id, Data: &driver.SpeakEvent{Stopped: true}})
}

// releaseSpeaking lets the utterance spoken go, its handlers with it.
func releaseSpeaking() {
	u := speaking.utterance
	u.Set("onend", js.Null())
	u.Set("onerror", js.Null())
	speaking.end.Release()
	speaking.fail.Release()
	speaking.id, speaking.utterance, speaking.dispatch = 0, js.Undefined(), nil
}

// webVoice is the voice of id, or the first voice of lang where the browser lists voices; undefined where it lists none, for the browser to choose by the utterance's lang, and "" or why it has none.
func webVoice(synth js.Value, lang, id string) (js.Value, string) {
	voices := synth.Call("getVoices")
	n := voices.Length()
	if id != "" {
		for i := range n {
			if v := voices.Index(i); v.Get("voiceURI").String() == id {
				return v, ""
			}
		}
		return js.Undefined(), driver.SpeakVoiceUnavailable
	}
	if n == 0 {
		return js.Undefined(), ""
	}
	lang = strings.ToLower(lang)
	var under js.Value
	for i := range n {
		v := voices.Index(i)
		tag := strings.ToLower(driver.LanguageTag(v.Get("lang").String()))
		switch {
		case tag == lang && v.Get("default").Bool():
			return v, ""
		case tag == lang || strings.HasPrefix(tag, lang+"-"):
			if !under.Truthy() {
				under = v
			}
		}
	}
	if !under.Truthy() {
		return js.Undefined(), driver.SpeakLanguageUnavailable
	}
	return under, ""
}

// webVoices are the browser's voices, by their voiceURI.
func webVoices(voices js.Value) []driver.Voice {
	out := make([]driver.Voice, 0, voices.Length())
	for i := range voices.Length() {
		v := voices.Index(i)
		out = append(out, driver.Voice{ID: v.Get("voiceURI").String(), Name: v.Get("name").String(), Lang: v.Get("lang").String()})
	}
	return out
}

// webSpeakEnd is the end a SpeechSynthesisErrorEvent's error says: a cancel or an interruption stops the utterance, and the rest fail it with the error of the same name.
func webSpeakEnd(code string) driver.SpeakEvent {
	switch code {
	case "canceled", "interrupted":
		return driver.SpeakEvent{Stopped: true}
	case driver.SpeakNotAllowed, driver.SpeakLanguageUnavailable, driver.SpeakVoiceUnavailable, driver.SpeakTextTooLong,
		driver.SpeakAudioBusy, driver.SpeakUnavailable:
		return driver.SpeakEvent{Error: code}
	}
	return driver.SpeakEvent{Error: driver.SpeakFailed}
}
