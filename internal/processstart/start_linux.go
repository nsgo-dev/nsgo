package processstart

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func procStart(path string) time.Time {
	stat, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	ticks, ok := procStartTicks(stat)
	if !ok {
		return time.Time{}
	}
	// /proc reports USER_HZ (100 on Linux and Android Go architectures),
	// not the kernel's configured interrupt frequency. Use CLOCK_BOOTTIME
	// for the boot offset: /proc/stat's btime only has whole seconds.
	const tick = time.Second / 100
	var boot unix.Timespec
	wall := time.Now()
	if unix.ClockGettime(unix.CLOCK_BOOTTIME, &boot) != nil {
		return time.Time{}
	}
	wall = wall.Add(time.Since(wall) / 2)
	uptime := time.Duration(boot.Nano())
	if uptime < 0 || ticks > int64(uptime/tick) {
		return time.Time{}
	}
	return wall.Add(time.Duration(ticks)*tick - uptime).UTC()
}
