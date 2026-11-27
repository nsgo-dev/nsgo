//go:build linux && !android

package processstart

import "time"

func read() time.Time { return procStart("/proc/self/stat") }
