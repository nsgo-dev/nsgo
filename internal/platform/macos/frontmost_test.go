//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func TestCoversDisplayTellsFullScreenFromZoomed(t *testing.T) {
	// A 1512x982 main display with a 30-point menu bar, and a 1920x1080 display to its right, its top 100 points higher.
	screens := []guideRect{{0, 0, 1512, 982}, {1512, -100, 1920, 1080}}
	for _, tc := range []struct {
		name   string
		window guideRect
		want   bool
	}{
		{"full screen", guideRect{0, 0, 1512, 982}, true},
		{"full screen on the second display", guideRect{1512, -100, 1920, 1080}, true},
		{"zoomed below the menu bar", guideRect{0, 30, 1512, 952}, false},
		{"across both displays without covering either", guideRect{800, 0, 1500, 900}, false},
		{"larger than its display", guideRect{-4, -4, 1520, 990}, true},
	} {
		if got := coversDisplay(tc.window, screens); got != tc.want {
			t.Errorf("%s: coversDisplay = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestReadFrontmostNamesTheWorkspacesApp(t *testing.T) {
	if !loadWindowList() {
		t.Skip("no window server")
	}
	onMain(func() {
		loadAppKit()
		withPool(func() {
			front := objc.ID(apple.Class("NSWorkspace")).Send(sel("sharedWorkspace")).Send(sel("frontmostApplication"))
			if front == 0 {
				t.Skip("no application in front, as outside a login session")
			}
			app := readFrontmost()
			if want := int(int32(apple.Msg(front, sel("processIdentifier")))); app.PID != want || app.PID <= 0 {
				t.Errorf("PID = %d, want %d", app.PID, want)
			}
			if app.ID == "" || app.Name == "" {
				t.Errorf("frontmost app %+v lacks its ID or name", app)
			}
		})
	})
}

// An activation or a change of Space reads the application in front again: a stale value gives way to what NSWorkspace says, from either notification.
func TestFrontmostFollowsWorkspaceNotifications(t *testing.T) {
	if !loadWindowList() {
		t.Skip("no window server")
	}
	onMain(func() {
		loadAppKit()
		withPool(func() {
			workspace := objc.ID(apple.Class("NSWorkspace")).Send(sel("sharedWorkspace"))
			if workspace.Send(sel("frontmostApplication")) == 0 {
				t.Skip("no application in front, as outside a login session")
			}
			frontmostApp()
			center := workspace.Send(sel("notificationCenter"))
			stale := driver.FrontmostApp{ID: "stale", Name: "stale", PID: -1}
			for _, name := range []string{"NSWorkspaceDidActivateApplicationNotification", "NSWorkspaceActiveSpaceDidChangeNotification"} {
				frontmost = stale
				center.Send(sel("postNotificationName:object:"), apple.Global(appKitPath, name), workspace)
				if want := readFrontmost(); frontmost != want {
					t.Errorf("after %s, frontmost = %+v, want %+v", name, frontmost, want)
				}
			}
		})
	})
}
