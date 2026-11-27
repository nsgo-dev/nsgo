//go:build js && wasm && !nsgominiprogram && !nsgoharmony

package processstart

import (
	"math"
	"syscall/js"
	"time"
)

func read() time.Time {
	performance := js.Global().Get("performance")
	if performance.Type() != js.TypeObject {
		return time.Time{}
	}
	origin := performance.Get("timeOrigin")
	if origin.Type() != js.TypeNumber {
		return time.Time{}
	}
	millis := origin.Float()
	if !(millis > 0) || math.IsInf(millis, 1) {
		return time.Time{}
	}
	whole, fraction := math.Modf(millis)
	return time.UnixMilli(int64(whole)).Add(time.Duration(fraction * float64(time.Millisecond))).UTC()
}
