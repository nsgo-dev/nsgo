//go:build ios

package audiosession

import (
	"errors"
	"log"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	avfaudio = "/System/Library/Frameworks/AVFAudio.framework/AVFAudio"
	// The AVAudioSessionCategoryOptions: MixWithOthers, AllowBluetoothHFP and DefaultToSpeaker.
	optionMix       = 0x1
	optionBluetooth = 0x4
	optionSpeaker   = 0x8
	// notifyOthers is AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation.
	notifyOthers = 1
)

var errSession = errors.New("audiosession: cannot set and activate the audio session")

// category is a session's category, mode and options.
type category struct {
	category, mode objc.ID
	options        uint
}

var system struct {
	once sync.Once
	// names are AVFAudio's category and mode constants, by name.
	names map[string]objc.ID
	// before is the category from before the first holder.
	before category
}

func sel(name string) objc.SEL { return objc.RegisterName(name) }

// constant reads the NSString AVFAudio exports as name, 0 without it.
func constant(name string) objc.ID {
	system.once.Do(func() {
		system.names = map[string]objc.ID{}
		lib, err := purego.Dlopen(avfaudio, purego.RTLD_NOW)
		if err != nil {
			return
		}
		for _, n := range []string{"AVAudioSessionCategoryAmbient", "AVAudioSessionCategoryPlayback", "AVAudioSessionCategoryPlayAndRecord",
			"AVAudioSessionModeDefault", "AVAudioSessionModeSpokenAudio", "AVAudioSessionModeMeasurement"} {
			if symbol, err := purego.Dlsym(lib, n); err == nil {
				system.names[n] = **(**objc.ID)(unsafe.Pointer(&symbol))
			}
		}
	})
	return system.names[name]
}

// of is the category level sets.
func of(level Level) category {
	switch level {
	case Ambient:
		return category{constant("AVAudioSessionCategoryAmbient"), constant("AVAudioSessionModeDefault"), 0}
	case Mixing:
		return category{constant("AVAudioSessionCategoryPlayback"), constant("AVAudioSessionModeSpokenAudio"), optionMix}
	case Playback:
		return category{constant("AVAudioSessionCategoryPlayback"), constant("AVAudioSessionModeSpokenAudio"), 0}
	case Recording:
		return category{constant("AVAudioSessionCategoryPlayAndRecord"), constant("AVAudioSessionModeDefault"), optionBluetooth | optionSpeaker}
	}
	return category{constant("AVAudioSessionCategoryPlayAndRecord"), constant("AVAudioSessionModeMeasurement"), optionBluetooth | optionSpeaker}
}

func setCategory(shared objc.ID, c category) bool {
	if c.category == 0 || c.mode == 0 {
		return false
	}
	return objc.Send[bool](shared, sel("setCategory:mode:options:error:"), c.category, c.mode, c.options, objc.ID(0))
}

// restore deactivates the session, telling the apps it interrupted that they may play, and gives back the category from before.
func restore(shared objc.ID) {
	objc.Send[bool](shared, sel("setActive:withOptions:error:"), false, uint(notifyOthers), objc.ID(0))
	if !setCategory(shared, system.before) {
		log.Print("nsgo: cannot restore the audio session")
	}
	for _, id := range []objc.ID{system.before.category, system.before.mode} {
		if id != 0 {
			id.Send(sel("release"))
		}
	}
	system.before = category{}
}

// apply moves the session from level from to level to, 0 being none held.
func apply(from, to Level) error {
	if constant("AVAudioSessionCategoryAmbient") == 0 {
		return errSession
	}
	shared := objc.ID(objc.GetClass("AVAudioSession")).Send(sel("sharedInstance"))
	if shared == 0 {
		return errSession
	}
	if to == 0 {
		restore(shared)
		return nil
	}
	if from == 0 {
		system.before = category{category: shared.Send(sel("category")).Send(sel("retain")),
			mode: shared.Send(sel("mode")).Send(sel("retain")), options: objc.Send[uint](shared, sel("categoryOptions"))}
	}
	if !setCategory(shared, of(to)) || !objc.Send[bool](shared, sel("setActive:error:"), true, objc.ID(0)) {
		if from == 0 {
			restore(shared)
		} else {
			setCategory(shared, of(from))
		}
		return errSession
	}
	return nil
}
