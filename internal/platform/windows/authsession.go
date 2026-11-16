//go:build windows

package windows

import (
	"github.com/nsgo-dev/nsgo/desktop/locales"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// authSession carries out an auth session's request in the default browser, which redirects to a loopback address the window serves: Windows has no system session, and the app's own scheme reaches no running app yet (docs/roadmap.md).
func (w *window) authSession(id uint32, r driver.AuthRequest) {
	open := func(u string, answer func(bool)) { answer(w.shellOpenURL(u)) }
	w.loopback.Handle(id, r, open, locales.In(w.language).AuthSessionDone(), w.engine.Dispatch)
}
