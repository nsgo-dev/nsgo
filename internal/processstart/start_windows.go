package processstart

import (
	"syscall"
	"time"
)

func read() time.Time {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return time.Time{}
	}
	var start, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(process, &start, &exit, &kernel, &user); err != nil || start == (syscall.Filetime{}) {
		return time.Time{}
	}
	return time.Unix(0, start.Nanoseconds()).UTC()
}
