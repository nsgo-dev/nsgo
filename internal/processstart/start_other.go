//go:build !darwin && !windows && !linux && !(js && wasm && !nsgominiprogram && !nsgoharmony)

package processstart

import "time"

func read() time.Time { return time.Time{} }
