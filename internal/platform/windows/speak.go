//go:build windows

package windows

import (
	"log"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Speaking through WinRT's SpeechSynthesizer, whose stream of the text spoken a MediaPlayer plays: one synthesizer and one player for the process, which speak one utterance at a time, a new one stopping the one before. Its state is the UI thread's; the synthesis's completion and the player's events, raised on worker threads, post to the window that spoke.

var (
	iidInstalledVoices     = parseGUID("7d526ecc-7533-4c3f-85be-888c2baeebdc")
	iidSpeechSynthesizer   = parseGUID("ce9f7c76-97f4-4ced-ad68-d51c458e45c6")
	iidSpeechSynthesizer2  = parseGUID("a7c5ecb2-4339-4d6a-bbf8-c7a4f1544c2e")
	iidSynthesizerOptions2 = parseGUID("1cbef60e-119c-4bed-b118-d250c3a25793")
	iidRandomAccessStream  = parseGUID("905a0fe1-bc53-11df-8c49-001e4fc686da")
	iidMediaSourceStatics  = parseGUID("f77d6fa4-4652-410e-b1d8-e9a5e245a45c")
	iidMediaPlaybackSource = parseGUID("ef9dc2bc-9317-4696-b051-2bad643177b5")
	iidMediaPlayer         = parseGUID("381a83cb-6fff-499b-8d64-2885dfc1249e")
	iidMediaPlayerSource2  = parseGUID("82449b9f-7322-4c0b-b03b-3e69a48260c5")
	// The parameterized interfaces' IDs, derived from their signatures as WinRT does (the namespace 11f47ad5-7b73-42c0-abae-878b1e16adee and SHA-1), as iidToastActivated is: AsyncOperationCompletedHandler<SpeechSynthesisStream>, TypedEventHandler<MediaPlayer, IInspectable>, MediaEnded's, and TypedEventHandler<MediaPlayer, MediaPlayerFailedEventArgs>, MediaFailed's.
	iidSynthesisCompleted = parseGUID("c972b996-6165-50d4-af60-a8c3df51d092")
	iidMediaEnded         = parseGUID("f1a6a51e-d078-5c40-ba3f-348870ba5c87")
	iidMediaFailed        = parseGUID("362c45a7-3a0a-5e27-99ce-cff6d1b770e1")
)

// The methods called, by their vtable slots after IInspectable's six.
const (
	synthesizerToStream    = 6
	synthesizerPutVoice    = 8
	synthesizer2Options    = 6
	optionsPutSpeakingRate = 9
	optionsPutAudioPitch   = 11
	voicesAll              = 6
	voicesDefault          = 7
	voiceDisplayName       = 6
	voiceID                = 7
	voiceLanguage          = 8
	asyncPutCompleted      = 6
	asyncGetResults        = 8
	mediaSourceFromStream  = 11
	playerPutAutoPlay      = 7
	playerAddMediaEnded    = 27
	playerAddMediaFailed   = 29
	playerPlay             = 45
	playerPause            = 46
	playerSource2PutSource = 7
	asyncStatusCompleted   = 1
	speakSynthesizerClass  = "Windows.Media.SpeechSynthesis.SpeechSynthesizer"
	speakMediaPlayerClass  = "Windows.Media.Playback.MediaPlayer"
	speakMediaSourceClass  = "Windows.Media.Core.MediaSource"
	speakStreamContentType = "audio/wav"
)

var speaking struct {
	synthesizer, player *comObject
	// run is the utterance being spoken, nil for none: a synthesis that completes after its utterance was stopped plays nothing.
	run *speakRun
	// source is what the player plays, released as it ends.
	source *comObject
}

type speakRun struct {
	w  *window
	id uint32
}

// speakAvailable is Environment.Speak: Windows has the SpeechSynthesizer class, from Windows 10.
var speakAvailable = sync.OnceValue(func() bool {
	comApartment()
	voices := activationFactory(speakSynthesizerClass, &iidInstalledVoices)
	voices.release()
	return voices != nil
})

// speak carries out an ItemSpeak of utterance or request id.
func (w *window) speak(id uint32, r driver.SpeakRequest) {
	report := func(e driver.SpeakEvent) {
		w.engine.Dispatch(driver.Event{Kind: driver.EventSpeak, Count: id, Data: &e})
	}
	switch r.Command {
	case driver.SpeakStart:
		stopSpeaking()
		if code := w.startSpeaking(id, r); code != "" {
			report(driver.SpeakEvent{Error: code})
		}
	case driver.SpeakStop:
		if run := speaking.run; run != nil && run.w == w && run.id == id {
			stopSpeaking()
		}
	case driver.SpeakVoices:
		report(driver.SpeakEvent{Voices: windowsVoices()})
	}
}

// startSpeaking synthesizes r's text, which the player plays once the stream is ready, or answers why it cannot, a driver Speak error.
func (w *window) startSpeaking(id uint32, r driver.SpeakRequest) string {
	comApartment()
	if speaking.synthesizer == nil {
		speaking.synthesizer = activate(speakSynthesizerClass, &iidSpeechSynthesizer)
		if speaking.synthesizer == nil {
			return driver.SpeakUnavailable
		}
	}
	voice, code := windowsVoice(r.Lang, r.Voice)
	if voice == nil {
		return code
	}
	hr := speaking.synthesizer.call(synthesizerPutVoice, voice.ptr())
	voice.release()
	if failed(hr) {
		return driver.SpeakVoiceUnavailable
	}
	// Rate and pitch from Windows 10 1709; earlier ones speak at the voice's own.
	if s2 := speaking.synthesizer.query(&iidSpeechSynthesizer2); s2 != nil {
		var options *comObject
		if !failed(s2.call(synthesizer2Options, uintptr(unsafe.Pointer(&options)))) {
			if o2 := options.query(&iidSynthesizerOptions2); o2 != nil {
				// SpeakingRate runs from 0.5 to 6 and AudioPitch from 0 to 2, 1 each the voice's own.
				o2.callDouble(optionsPutSpeakingRate, float64(driver.SpeakScale(r.Rate, 0.5, 1, 2)))
				o2.callDouble(optionsPutAudioPitch, float64(driver.SpeakScale(r.Pitch, 0.5, 1, 2)))
				o2.release()
			}
			options.release()
		}
		s2.release()
	}
	text := hstring(r.Text)
	var op *comObject
	hr = speaking.synthesizer.call(synthesizerToStream, text, uintptr(unsafe.Pointer(&op)))
	deleteHString(text)
	if failed(hr) {
		log.Print(hresultError("nsgo: speak: SynthesizeTextToStreamAsync", hr))
		return driver.SpeakFailed
	}
	run := &speakRun{w: w, id: id}
	setSpeakingRun(run)
	completed := newHandler(&iidSynthesisCompleted, func(_ uintptr, status unsafe.Pointer) {
		ok := uintptr(status) == asyncStatusCompleted
		var stream *comObject
		if ok && failed(op.call(asyncGetResults, uintptr(unsafe.Pointer(&stream)))) {
			ok = false
		}
		op.release()
		w.post(func() {
			defer stream.release()
			if speaking.run != run {
				return
			}
			if !ok || !play(stream) {
				endSpeaking(driver.SpeakEvent{Error: driver.SpeakFailed})
			}
		})
	})
	completed.agile = true
	hr = op.call(asyncPutCompleted, completed.ptr())
	completed.release()
	if failed(hr) {
		op.release()
		setSpeakingRun(nil)
		return driver.SpeakFailed
	}
	return ""
}

// play has the player play stream, the synthesized speech, making the player at the first.
func play(stream *comObject) bool {
	if speaking.player == nil && !makePlayer() {
		return false
	}
	ras := stream.query(&iidRandomAccessStream)
	statics := activationFactory(speakMediaSourceClass, &iidMediaSourceStatics)
	if ras == nil || statics == nil {
		ras.release()
		statics.release()
		return false
	}
	var source *comObject
	contentType := hstring(speakStreamContentType)
	hr := statics.call(mediaSourceFromStream, ras.ptr(), contentType, uintptr(unsafe.Pointer(&source)))
	deleteHString(contentType)
	ras.release()
	statics.release()
	if failed(hr) {
		return false
	}
	playable := source.query(&iidMediaPlaybackSource)
	player2 := speaking.player.query(&iidMediaPlayerSource2)
	if playable == nil || player2 == nil || failed(player2.call(playerSource2PutSource, playable.ptr())) {
		playable.release()
		player2.release()
		source.release()
		return false
	}
	playable.release()
	player2.release()
	releaseSource()
	speaking.source = source
	return !failed(speaking.player.call(playerPlay))
}

// makePlayer makes the process's MediaPlayer, whose ends and failures end the utterance playing.
func makePlayer() bool {
	player := activate(speakMediaPlayerClass, &iidMediaPlayer)
	if player == nil {
		return false
	}
	player.call(playerPutAutoPlay, 0)
	// On a worker thread: the ends go to the window that spoke, to end the utterance that is still the one playing.
	ended := func(e driver.SpeakEvent) func(uintptr, unsafe.Pointer) {
		return func(uintptr, unsafe.Pointer) {
			run := speakingRun()
			if run == nil {
				return
			}
			run.w.post(func() {
				if speaking.run == run {
					endSpeaking(e)
				}
			})
		}
	}
	for _, h := range []struct {
		iid    *guid
		method int
		end    driver.SpeakEvent
	}{{&iidMediaEnded, playerAddMediaEnded, driver.SpeakEvent{}}, {&iidMediaFailed, playerAddMediaFailed, driver.SpeakEvent{Error: driver.SpeakFailed}}} {
		handler := newHandler(h.iid, ended(h.end))
		handler.agile = true
		var token eventToken
		player.call(h.method, handler.ptr(), uintptr(unsafe.Pointer(&token)))
		handler.release()
	}
	speaking.player = player
	return true
}

// speakingMu guards speaking.run for the player's events, which read it on worker threads.
var speakingMu sync.Mutex

func speakingRun() *speakRun {
	speakingMu.Lock()
	defer speakingMu.Unlock()
	return speaking.run
}

// setSpeakingRun sets speaking.run, on the UI thread.
func setSpeakingRun(run *speakRun) {
	speakingMu.Lock()
	speaking.run = run
	speakingMu.Unlock()
}

// stopSpeaking stops the utterance being spoken, which reports itself stopped.
func stopSpeaking() {
	if speaking.run == nil {
		return
	}
	if speaking.player != nil {
		speaking.player.call(playerPause)
	}
	endSpeaking(driver.SpeakEvent{Stopped: true})
}

// endSpeaking ends the utterance being spoken, reporting e of it, and lets its stream go.
func endSpeaking(e driver.SpeakEvent) {
	run := speaking.run
	setSpeakingRun(nil)
	if speaking.player != nil {
		if player2 := speaking.player.query(&iidMediaPlayerSource2); player2 != nil {
			player2.call(playerSource2PutSource, 0)
			player2.release()
		}
	}
	releaseSource()
	if run != nil && run.id != menuUtterance {
		run.w.engine.Dispatch(driver.Event{Kind: driver.EventSpeak, Count: run.id, Data: &e})
	}
}

func releaseSource() {
	speaking.source.release()
	speaking.source = nil
}

// activate makes an instance of WinRT class name as interface iid, nil where Windows has none.
func activate(name string, iid *guid) *comObject {
	class := hstring(name)
	defer deleteHString(class)
	var inspectable *comObject
	if hr, _, _ := roActivateInstance.Call(class, uintptr(unsafe.Pointer(&inspectable))); failed(hr) {
		return nil
	}
	defer inspectable.release()
	return inspectable.query(iid)
}

// goHString copies a WinRT string, which the caller still frees.
func goHString(h uintptr) string {
	if h == 0 {
		return ""
	}
	var n uint32
	p, _, _ := windowsGetStringRawBuffer.Call(h, uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	// The buffer is the system's, outside Go's heap, so the word is read back as a pointer through memory, which vet's check of uintptr conversions leaves alone.
	buf := *(**uint16)(unsafe.Pointer(&p))
	return string(utf16.Decode(unsafe.Slice(buf, n)))
}

// voiceString reads one of a voice's strings, by its getter's slot.
func voiceString(voice *comObject, method int) string {
	var h uintptr
	if failed(voice.call(method, uintptr(unsafe.Pointer(&h)))) {
		return ""
	}
	defer deleteHString(h)
	return goHString(h)
}

// eachVoice calls fn with each voice installed, which fn may keep by adding a reference, until fn returns false.
func eachVoice(fn func(voice *comObject) bool) {
	statics := activationFactory(speakSynthesizerClass, &iidInstalledVoices)
	if statics == nil {
		return
	}
	defer statics.release()
	var all *comObject
	if failed(statics.call(voicesAll, uintptr(unsafe.Pointer(&all)))) {
		return
	}
	defer all.release()
	var n uint32
	all.call(vectorViewSize, uintptr(unsafe.Pointer(&n)))
	for i := range n {
		var v *comObject
		if failed(all.call(vectorViewGetAt, uintptr(i), uintptr(unsafe.Pointer(&v)))) {
			continue
		}
		more := fn(v)
		v.release()
		if !more {
			return
		}
	}
}

// windowsVoices lists the voices installed.
func windowsVoices() []driver.Voice {
	comApartment()
	var voices []driver.Voice
	eachVoice(func(v *comObject) bool {
		voices = append(voices, driver.Voice{ID: voiceString(v, voiceID), Name: voiceString(v, voiceDisplayName), Lang: voiceString(v, voiceLanguage)})
		return true
	})
	return voices
}

// windowsVoice is the voice of id, or the voice for lang, the default one where it speaks lang or lang is "", with a reference the caller releases; else nil and why.
func windowsVoice(lang, id string) (*comObject, string) {
	var found *comObject
	speaks := func(v *comObject) bool {
		tag := strings.ToLower(voiceString(v, voiceLanguage))
		l := strings.ToLower(lang)
		return l == "" || tag == l || strings.HasPrefix(tag, l+"-")
	}
	if id == "" {
		if statics := activationFactory(speakSynthesizerClass, &iidInstalledVoices); statics != nil {
			var def *comObject
			if !failed(statics.call(voicesDefault, uintptr(unsafe.Pointer(&def)))) {
				if speaks(def) {
					found = def
				} else {
					def.release()
				}
			}
			statics.release()
		}
		if found != nil {
			return found, ""
		}
	}
	eachVoice(func(v *comObject) bool {
		if id != "" && voiceString(v, voiceID) == id || id == "" && speaks(v) {
			v.addRef()
			found = v
			return false
		}
		return true
	})
	switch {
	case found != nil:
		return found, ""
	case id != "":
		return nil, driver.SpeakVoiceUnavailable
	}
	return nil, driver.SpeakLanguageUnavailable
}
