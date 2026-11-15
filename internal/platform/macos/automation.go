//go:build darwin && !ios && nsgoautomation

package macos

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// automationBuild is set in `nsgo dev -automation`'s builds: surfaces keep their alerts for the automation to answer, and it acts on the running app's windows through liveSurface.
const automationBuild = true

func init() { testhost.RegisterLive(liveSurface) }

// liveSurface returns the running app's surface id as a test host's surface acts on it: the app's own for 0, which shows no window but answers its alerts and chooses the main menu's items, else the window or panel the app opened as id; nil for a tray icon's view or a surface gone.
func liveSurface(id uint32) testhost.Surface {
	var found *surface
	onMainSync(func() {
		if own == nil {
			return
		}
		if id == 0 {
			found = own
			return
		}
		for s := range surfaces {
			if s.id == id && !s.embedded && !s.closed && s.window != 0 && s.kind != driver.SurfaceTrayIcon {
				found = s
			}
		}
	})
	if found == nil {
		return nil
	}
	return &testSurface{s: found}
}
