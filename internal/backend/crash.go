package backend

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/nsgo-dev/nsgo/crash"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// installCrashReporting installs package crash for this launch before the
// platform event loop starts, promoting any non-empty crash files from an
// earlier run and wiring uncaught engine panics into crash.RecordPanic.
func installCrashReporting() {
	var logs string
	switch runtime.GOOS {
	case "android":
		if data := os.Getenv("NSGO_DATA_DIR"); data != "" {
			logs = filepath.Join(data, "logs")
		}
	case "js":
		logs = ""
	default:
		_, _, logs = engine.AppDirs(id)
	}
	dir := crash.DirForLogs(logs)
	if dir == "" && runtime.GOOS != "js" {
		return
	}
	if err := crash.Install(crash.Options{Dir: dir}); err != nil {
		log.Printf("nsgo: crash reporting: %v", err)
		return
	}
	engine.OnPanic = func(value any, stack []byte) {
		_ = crash.RecordPanic(value, stack)
	}
}
