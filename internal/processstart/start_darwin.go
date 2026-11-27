package processstart

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func read() time.Time {
	proc, err := unix.SysctlKinfoProc("kern.proc.pid", os.Getpid())
	if err != nil || proc.Proc.P_starttime.Sec <= 0 {
		return time.Time{}
	}
	start := proc.Proc.P_starttime
	return time.Unix(start.Sec, int64(start.Usec)*1000).UTC()
}
