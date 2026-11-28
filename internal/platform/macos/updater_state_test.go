//go:build darwin && !ios && !nsgo_steam

package macos

import (
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/updates"
)

// Exercise the real ObjC property bridge without downloading an updater or checking a feed. Sparkle's own persistence remains behind those selectors.
func TestSparkleStateAndPreferencesUseNativeProperties(t *testing.T) {
	onMain(func() {
		old, config, failure := sparkle, sparkleConfig, sparkleFailure
		defer func() { sparkle, sparkleConfig, sparkleFailure = old, config, failure }()
		checks, downloads := true, false
		class := register("NsgoUpdaterStateFixture", apple.Class("NSObject"), nil, []objc.MethodDef{
			{Cmd: sel("updater"), Fn: func(self objc.ID, _ objc.SEL) objc.ID { return self }},
			{Cmd: sel("automaticallyChecksForUpdates"), Fn: func(_ objc.ID, _ objc.SEL) bool { return checks }},
			{Cmd: sel("automaticallyDownloadsUpdates"), Fn: func(_ objc.ID, _ objc.SEL) bool { return downloads }},
			{Cmd: sel("setAutomaticallyChecksForUpdates:"), Fn: func(_ objc.ID, _ objc.SEL, on bool) { checks = on }},
			{Cmd: sel("setAutomaticallyDownloadsUpdates:"), Fn: func(_ objc.ID, _ objc.SEL, on bool) { downloads = on }},
			{Cmd: sel("lastUpdateCheckDate"), Fn: func(_ objc.ID, _ objc.SEL) objc.ID {
				return objc.ID(apple.Class("NSDate")).Send(sel("dateWithTimeIntervalSince1970:"), float64(12345))
			}},
		})
		fixture := objc.ID(class).Send(selAlloc).Send(sel("init"))
		defer fixture.Send(selRelease)
		sparkle = fixture
		sparkleConfig = updates.Config{Channel: "direct", Feed: "https://example.invalid/feed"}
		sparkleFailure = ""
		if state := sparkleStatus(); !state.AutomaticChecks || state.AutomaticDownloads || state.LastCheck.Unix() != 12345 || state.Managed != driver.UpdateManagedSelf {
			t.Errorf("native state=%+v", state)
		}
		off, on := false, true
		(&surface{}).setUpdatePreference(&driver.UpdatePreference{AutomaticChecks: &off, AutomaticDownloads: &on})
		if state := sparkleStatus(); state.AutomaticChecks || !state.AutomaticDownloads {
			t.Errorf("native preferences=%+v", state)
		}
		sparkle = 0
		sparkleFailure = "framework load failed"
		if state := sparkleStatus(); state.Managed != driver.UpdateManagedUnavailable {
			t.Errorf("load failure=%+v", state)
		}
	})
}

func TestStandaloneAppStartsWithInitialUpdaterState(t *testing.T) {
	onMain(func() {
		loadAppKit()
		old, config, failure, prevOwn := sparkle, sparkleConfig, sparkleFailure, own
		defer func() {
			sparkle, sparkleConfig, sparkleFailure, own = old, config, failure, prevOwn
		}()
		class := register("NsgoStandaloneUpdaterFixture", apple.Class("NSObject"), nil, []objc.MethodDef{
			{Cmd: sel("updater"), Fn: func(self objc.ID, _ objc.SEL) objc.ID { return self }},
			{Cmd: sel("automaticallyChecksForUpdates"), Fn: func(_ objc.ID, _ objc.SEL) bool { return true }},
			{Cmd: sel("automaticallyDownloadsUpdates"), Fn: func(_ objc.ID, _ objc.SEL) bool { return false }},
			{Cmd: sel("lastUpdateCheckDate"), Fn: func(_ objc.ID, _ objc.SEL) objc.ID { return 0 }},
		})
		fixture := objc.ID(class).Send(selAlloc).Send(sel("init"))
		defer fixture.Send(selRelease)
		sparkle = fixture
		sparkleConfig = updates.Config{Channel: "direct", Feed: "https://example.invalid/feed"}
		sparkleFailure = ""
		gotCh := make(chan core.UpdaterState, 1)
		s := startOwn(func(cx *core.Scope) core.Node {
			gotCh <- core.UpdaterStatus(cx)
			return core.View{}
		}, Options{ID: "dev.nsgo.test.updater", Width: 200, Height: 100, Hidden: true})
		defer func() {
			s.close()
			own = nil
		}()
		select {
		case got := <-gotCh:
			if got.Managed != core.UpdaterSelf || !got.NativePreferences || !got.AutomaticChecks || got.AutomaticDownloads {
				t.Errorf("standalone initial updater state = %+v, want Managed=self NativePreferences=true AutomaticChecks=true", got)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("standalone app did not render initial frame")
		}
	})
}
