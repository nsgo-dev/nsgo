package core

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// speakItems are the batch's ItemSpeak requests, with their counts.
func speakItems(b *driver.MountBatch) (out []string) {
	for _, it := range b.Items {
		if it.Kind == driver.ItemSpeak {
			out = append(out, fmt.Sprintf("%s %d", it.Speak.Command, it.Count))
		}
	}
	return out
}

func newSpeakHost(t *testing.T, app func(c *Scope) Node) *host {
	h := newHost(t, Size{Width: 100, Height: 100}, app)
	h.tree.SetEnvironment(Environment{Speak: true, Locale: "en-US"})
	h.commit()
	return h
}

func TestSpeakUtterances(t *testing.T) {
	var cx *Scope
	h := newSpeakHost(t, func(c *Scope) Node { cx = c; return View{} })
	var log []string
	done := func(name string) func(bool, error) {
		return func(stopped bool, err error) {
			log = append(log, fmt.Sprintf("%s stopped=%v err=%v", name, stopped, err))
		}
	}
	report := func(id uint32, e driver.SpeakEvent) {
		h.tree.Dispatch(driver.Event{Kind: driver.EventSpeak, Count: id, Data: &e})
	}

	if err := cx.Speak("Hello", Voice{Rate: 3, Pitch: 0.25}, done("hello")); err != nil {
		t.Fatal(err)
	}
	it := command(h.commit(), driver.ItemSpeak)
	if it == nil || it.Speak.Command != driver.SpeakStart || it.Speak.Text != "Hello" || it.Speak.Lang != "en-US" ||
		it.Speak.Rate != 2 || it.Speak.Pitch != 0.5 {
		t.Fatalf("start = %+v", it)
	}
	report(it.Count, driver.SpeakEvent{})
	report(it.Count, driver.SpeakEvent{Error: driver.SpeakFailed}) // after the end
	if want := []string{"hello stopped=false err=<nil>"}; !slices.Equal(log, want) {
		t.Fatalf("log %q, want %q", log, want)
	}

	// A second utterance stops the first, whose done hears it stopped at once, and the platform's late report of it is ignored.
	log = nil
	cx.Speak("one", Voice{Lang: "ja-JP", ID: "kyoko"}, done("one"))
	first := command(h.commit(), driver.ItemSpeak)
	if first.Speak.Lang != "ja-JP" || first.Speak.Voice != "kyoko" || first.Speak.Rate != 1 || first.Speak.Pitch != 1 {
		t.Fatalf("voice = %+v", first.Speak)
	}
	cx.Speak("two", Voice{}, done("two"))
	b := h.commit()
	second := b.Items[slices.IndexFunc(b.Items, func(it driver.MountItem) bool {
		return it.Kind == driver.ItemSpeak && it.Speak.Command == driver.SpeakStart
	})]
	if got, want := speakItems(b), []string{fmt.Sprintf("stop %d", first.Count), fmt.Sprintf("start %d", second.Count)}; !slices.Equal(got, want) {
		t.Fatalf("items %q, want %q", got, want)
	}
	report(first.Count, driver.SpeakEvent{Stopped: true})
	if want := []string{"one stopped=true err=<nil>"}; !slices.Equal(log, want) {
		t.Fatalf("log %q, want %q", log, want)
	}

	// StopSpeaking stops it where it is; a failure ends one with its error.
	cx.StopSpeaking()
	if got := speakItems(h.commit()); !slices.Equal(got, []string{fmt.Sprintf("stop %d", second.Count)}) {
		t.Fatalf("stop items %q", got)
	}
	cx.StopSpeaking()
	if got := speakItems(h.commit()); got != nil {
		t.Fatalf("a second stop sent %q", got)
	}
	var failed error
	cx.Speak("three", Voice{}, func(_ bool, err error) { failed = err })
	third := command(h.commit(), driver.ItemSpeak)
	report(third.Count, driver.SpeakEvent{Error: driver.SpeakLanguageUnavailable})
	if !errors.Is(failed, SpeakLanguageUnavailable) {
		t.Fatalf("failed with %v", failed)
	}
	if want := []string{"one stopped=true err=<nil>", "two stopped=true err=<nil>"}; !slices.Equal(log, want) {
		t.Fatalf("log %q, want %q", log, want)
	}

	if err := cx.Speak("x", Voice{Lang: "not a tag"}, nil); err == nil {
		t.Error("a Lang that is no tag speaks")
	}
	// Nothing to say finishes without asking the platform.
	var blank []string
	cx.Speak("  ", Voice{}, func(stopped bool, err error) { blank = append(blank, fmt.Sprint(stopped, err)) })
	if got := speakItems(h.commit()); got != nil {
		t.Fatalf("blank text sent %q", got)
	}
	h.drain()
	if !slices.Equal(blank, []string{"false <nil>"}) {
		t.Fatalf("blank text ended %q", blank)
	}
}

func TestSpeakStopsWithItsScope(t *testing.T) {
	var cx *Scope
	h := newSpeakHost(t, func(c *Scope) Node { cx = c; return View{} })
	child := cx.Child()
	var ended []string
	child.Speak("bye", Voice{}, func(stopped bool, err error) { ended = append(ended, fmt.Sprint(stopped, err)) })
	it := command(h.commit(), driver.ItemSpeak)
	child.Dispose()
	if got := speakItems(h.commit()); !slices.Equal(got, []string{fmt.Sprintf("stop %d", it.Count)}) {
		t.Fatalf("items %q", got)
	}
	if !slices.Equal(ended, []string{"true <nil>"}) {
		t.Fatalf("ended %q", ended)
	}
}

func TestSpeakUnavailable(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var ended error
	cx.Speak("Hello", Voice{}, func(_ bool, err error) { ended = err })
	var voices []VoiceInfo
	answered := false
	cx.Voices("", func(v []VoiceInfo) { voices, answered = v, true })
	if got := speakItems(h.commit()); got != nil {
		t.Fatalf("sent %q", got)
	}
	h.drain()
	if !errors.Is(ended, SpeakUnavailable) || !answered || voices != nil {
		t.Fatalf("ended with %v, voices %v %v", ended, answered, voices)
	}
}

func TestSpeakHeldToAccess(t *testing.T) {
	var cx *Scope
	h := newSpeakHost(t, func(c *Scope) Node { cx = c; return View{} })
	refused, granted := cx.Child(), cx.Child()
	refused.ProvideAccess(allows(func(AccessRequest) bool { return false }))
	granted.ProvideAccess(allows(func(r AccessRequest) bool { return r.Permission == AccessTextToSpeech }))
	var ended []error
	refused.Speak("no", Voice{}, func(_ bool, err error) { ended = append(ended, err) })
	answered := false
	refused.Voices("", func(v []VoiceInfo) { answered = v == nil })
	if got := speakItems(h.commit()); got != nil {
		t.Fatalf("refused code sent %q", got)
	}
	h.drain()
	if len(ended) != 1 || !errors.Is(ended[0], SpeakNotAllowed) || !answered {
		t.Fatalf("refused code ended %v, voices answered %v", ended, answered)
	}
	granted.Speak("yes", Voice{}, nil)
	if got := speakItems(h.commit()); len(got) != 1 {
		t.Fatalf("granted code sent %q", got)
	}
}

func TestVoices(t *testing.T) {
	var cx *Scope
	h := newSpeakHost(t, func(c *Scope) Node { cx = c; return View{} })
	var got []string
	cx.Voices("en", func(voices []VoiceInfo) {
		for _, v := range voices {
			got = append(got, v.Lang+" "+v.Name)
		}
	})
	it := command(h.commit(), driver.ItemSpeak)
	if it == nil || it.Speak.Command != driver.SpeakVoices {
		t.Fatalf("request = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventSpeak, Count: it.Count, Data: &driver.SpeakEvent{Voices: []driver.Voice{
		{ID: "3", Name: "Samantha", Lang: "en-US"}, {ID: "1", Name: "Kyoko", Lang: "ja-JP"}, {ID: "2", Name: "Daniel", Lang: "en_GB"},
		{ID: "4", Name: "Alex", Lang: "en-US"}, {ID: "5", Name: "Eng", Lang: "eng"},
	}}})
	if want := []string{"en-GB Daniel", "en-US Alex", "en-US Samantha"}; !slices.Equal(got, want) {
		t.Fatalf("voices %q, want %q", got, want)
	}
}
