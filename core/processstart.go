package core

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/processstart"
)

// ProcessStart returns the process's launch time on the wall clock, even
// before nsgo.Run. On Android it is the Java host's launch, and on the web
// it is the page's time origin, before the WebAssembly app starts. The
// reading stays fixed for the life of the process. It is zero on unsupported
// targets or where the platform's launch time is unavailable.
func ProcessStart() time.Time { return processstart.Time() }
