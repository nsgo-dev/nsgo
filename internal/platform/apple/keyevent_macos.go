//go:build darwin && !ios

package apple

import (
	"runtime"

	"github.com/ebitengine/purego/objc"
)

// KeyEvent makes an AppKit key event of kind (NSEventTypeKeyDown or NSEventTypeKeyUp) for window, typing chars and ignoring without modifiers. AppKit must be loaded.
//
// isARepeat and keyCode are the selector's last arguments and go on the stack, past the integer registers. Apple's arm64 ABI packs them by their natural size into one word, isARepeat in its low byte and keyCode from its third, which purego, giving each variadic argument a word of its own, would not; the x86-64 ABI gives each its own word, as purego does.
func KeyEvent(kind, flags uint, timestamp float64, window int, chars, ignoring string, repeat bool, code uint16) objc.ID {
	args := []any{kind, CGPoint{}, flags, timestamp, window, objc.ID(0), NSString(chars), NSString(ignoring)}
	if runtime.GOARCH == "arm64" {
		var packed uint64
		if repeat {
			packed = 1
		}
		args = append(args, packed|uint64(code)<<16)
	} else {
		args = append(args, repeat, code)
	}
	return objc.ID(Class("NSEvent")).Send(
		Sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
		args...)
}
