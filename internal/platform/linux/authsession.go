//go:build linux && !android

package linux

import (
	"github.com/nsgo-dev/nsgo/desktop/locales"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// authSession carries out an auth session's request in the default browser, which redirects to a loopback address the window serves: Linux has no system session, and the app's own scheme reaches no running app yet (docs/roadmap.md).
func (w *window) authSession(id uint32, r driver.AuthRequest) {
	w.loopback.Handle(id, r, w.launchURI, locales.In(w.language).AuthSessionDone(), w.engine.Dispatch)
}
