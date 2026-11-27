package processstart

import (
	"os"
	"strconv"
	"time"
)

func read() time.Time {
	if millis, err := strconv.ParseInt(os.Getenv("NSGO_PROCESS_START_MS"), 10, 64); err == nil && millis > 0 {
		return time.UnixMilli(millis).UTC()
	}
	// The previous Java runtime doesn't export a timestamp, but it sets
	// NSGO_DATA_DIR and starts Go directly. Read the Go child's parent
	// rather than reporting the later Go process or an unrelated shell.
	if parent := os.Getppid(); parent > 1 && os.Getenv("NSGO_DATA_DIR") != "" {
		return procStart("/proc/" + strconv.Itoa(parent) + "/stat")
	}
	return time.Time{}
}
