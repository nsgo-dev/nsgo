//go:build darwin && !ios

package macos

import (
	"math"

	"github.com/ebitengine/purego/objc"
)

// testInputFilter is installed only by the native test host, on the main thread. Live automation and ordinary applications leave it nil.
var testInputFilter func(objc.ID) bool

// inputAllowed keeps AppKit's own window/application events while a test takes only its scripted pointer, keyboard and gesture events. A nil event is the host's direct hover enter/leave, not hardware input.
func inputAllowed(event objc.ID) bool {
	if testInputFilter == nil || event == 0 {
		return true
	}
	kind := objc.Send[uint](event, sel("type"))
	if kind >= 1 && kind <= 12 || kind >= 17 && kind <= 20 || kind >= 22 && kind <= 34 || kind == 37 || kind == 38 {
		return testInputFilter(event)
	}
	return true
}

// AppKit can copy a posted event, so object identity and eventNumber do not identify it. Like NSH's synth, keep a bounded history of its type and nanosecond timestamp; also match the window so another window can never use its record. Only the main thread reads or writes this cache.
type scriptedInputs struct {
	events      [64]scriptedInput
	next, count int
}

type scriptedInput struct {
	kind   uint
	window int
	at     int64
}

func inputIdentity(event objc.ID) scriptedInput {
	return scriptedInput{kind: objc.Send[uint](event, sel("type")),
		window: objc.Send[int](event, sel("windowNumber")),
		at:     int64(math.Round(objc.Send[float64](event, sel("timestamp")) * 1e9))}
}

func (c *scriptedInputs) remember(event objc.ID) {
	c.events[c.next] = inputIdentity(event)
	c.next = (c.next + 1) % len(c.events)
	c.count = min(c.count+1, len(c.events))
}

func (c *scriptedInputs) contains(event objc.ID) bool {
	want := inputIdentity(event)
	for _, got := range c.events[:c.count] {
		delta := got.at - want.at
		if got.kind == want.kind && got.window == want.window && delta >= -1 && delta <= 1 {
			return true
		}
	}
	return false
}

var scripted scriptedInputs

func rememberTestInput(event objc.ID) {
	if testInputFilter != nil {
		scripted.remember(event)
	}
}
