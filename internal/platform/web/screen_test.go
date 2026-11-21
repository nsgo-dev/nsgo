package web

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// fakeBrowserScreen simulates browser Fullscreen and Screen Orientation APIs
// (Chrome on Android, Safari on iOS/macOS, legacy WebKit) without a real browser.
type fakeBrowserScreen struct {
	fullScreenEnabled bool
	fullScreenMethod  bool // has requestFullscreen / webkitRequestFullscreen
	legacyWebKit      bool // requestFullscreen returns undefined instead of Promise
	fullScreen        bool

	// requireGesture causes requestFullscreen to reject with TypeError when userGesture is false.
	requireGesture bool
	userGesture    bool

	// fullScreenErr, when non-nil, forces requestFullscreen/exitFullscreen to fail with that error.
	fullScreenErr *screenError

	orientationSupported bool
	// requireFullScreenForOrientation simulates Chrome on Android requiring full screen before screen.orientation.lock succeeds.
	requireFullScreenForOrientation bool
	orientation                     driver.Orientation
	orientationLockErr              *screenError
	unlockErr                       *screenError

	// pending async promise callbacks (settled explicitly by tests to check ordering and deferral).
	deferFullScreen  bool
	pendingFS        func(*screenError)
	deferOrientation bool
	pendingOrient    []func(*screenError)

	// trace records synchronous browser calls in order.
	calls   []string
	locked  string
	unlocks int

	envUpdates int
	events     []driver.Event
	onEnv      func(driver.Environment)
	onDispatch func(driver.Event)
}

func (f *fakeBrowserScreen) canFullScreen() bool {
	return f.fullScreenMethod && f.fullScreenEnabled
}

func (f *fakeBrowserScreen) isFullScreen() bool { return f.fullScreen }

func (f *fakeBrowserScreen) requestFullScreen(settle func(*screenError)) (bool, *screenError) {
	f.calls = append(f.calls, "requestFullscreen")
	if !f.fullScreenMethod {
		return false, &screenError{Name: "NotSupportedError", Message: "full screen is not supported by this browser"}
	}
	if f.legacyWebKit {
		return false, nil
	}
	complete := func() *screenError {
		if f.fullScreenErr != nil {
			return f.fullScreenErr
		}
		if !f.fullScreenEnabled {
			return &screenError{Name: "TypeError", Message: "Disallowed by permissions policy"}
		}
		if f.requireGesture && !f.userGesture {
			return &screenError{Name: "TypeError", Message: "Failed to execute 'requestFullscreen' on 'Element': API can only be initiated by a user gesture."}
		}
		f.fullScreen = true
		return nil
	}
	if f.deferFullScreen {
		// Capture user-gesture validity synchronously at call time, as browsers do.
		err := complete()
		if err == nil {
			f.fullScreen = false // becomes true only when the promise resolves
		}
		f.pendingFS = func(override *screenError) {
			if override != nil {
				settle(override)
				return
			}
			if err == nil {
				f.fullScreen = true
			}
			settle(err)
		}
		return true, nil
	}
	settle(complete())
	return true, nil
}

func (f *fakeBrowserScreen) exitFullScreen(settle func(*screenError)) (bool, *screenError) {
	f.calls = append(f.calls, "exitFullscreen")
	if !f.fullScreenMethod {
		return false, &screenError{Name: "NotSupportedError", Message: "full screen exit is not supported by this browser"}
	}
	if f.legacyWebKit {
		return false, nil
	}
	if f.deferFullScreen {
		f.pendingFS = func(override *screenError) {
			if override != nil {
				settle(override)
				return
			}
			if f.fullScreenErr != nil {
				settle(f.fullScreenErr)
				return
			}
			f.fullScreen = false
			settle(nil)
		}
		return true, nil
	}
	if f.fullScreenErr != nil {
		settle(f.fullScreenErr)
		return true, nil
	}
	f.fullScreen = false
	settle(nil)
	return true, nil
}

func (f *fakeBrowserScreen) canLockOrientation() bool { return f.orientationSupported }

func (f *fakeBrowserScreen) currentOrientation() driver.Orientation { return f.orientation }

func (f *fakeBrowserScreen) lockOrientation(lockType string, settle func(*screenError)) (bool, *screenError) {
	f.calls = append(f.calls, "orientation.lock("+lockType+")")
	if !f.orientationSupported {
		return false, &screenError{Name: "NotSupportedError", Message: "screen orientation lock is not supported by this browser"}
	}
	resolve := func() *screenError {
		if f.orientationLockErr != nil {
			return f.orientationLockErr
		}
		if f.requireFullScreenForOrientation && !f.fullScreen {
			return &screenError{Name: "SecurityError", Message: "The document must be in fullscreen mode."}
		}
		f.locked = lockType
		switch lockType {
		case "portrait", "portrait-primary":
			f.orientation = driver.OrientationPortrait
		case "portrait-secondary":
			f.orientation = driver.OrientationPortraitUpsideDown
		case "landscape", "landscape-primary":
			f.orientation = driver.OrientationLandscapeLeft
		case "landscape-secondary":
			f.orientation = driver.OrientationLandscapeRight
		}
		return nil
	}
	if f.deferOrientation {
		f.pendingOrient = append(f.pendingOrient, func(override *screenError) {
			if override != nil {
				settle(override)
				return
			}
			settle(resolve())
		})
		return true, nil
	}
	settle(resolve())
	return true, nil
}

func (f *fakeBrowserScreen) unlockOrientation() *screenError {
	f.calls = append(f.calls, "orientation.unlock")
	f.unlocks++
	f.locked = ""
	return f.unlockErr
}

func (f *fakeBrowserScreen) environment() driver.Environment {
	return driver.Environment{
		OS:                 driver.OSWeb,
		FullScreen:         f.isFullScreen(),
		CanFullScreen:      f.canFullScreen(),
		CanLockOrientation: f.canLockOrientation(),
		Orientation:        f.currentOrientation(),
	}
}

func (f *fakeBrowserScreen) updateEnv() {
	f.envUpdates++
	if f.onEnv != nil {
		f.onEnv(f.environment())
	}
}

func (f *fakeBrowserScreen) dispatch(ev driver.Event) {
	f.events = append(f.events, ev)
	if f.onDispatch != nil {
		f.onDispatch(ev)
	}
}

func TestOrientationLockTypeMapping(t *testing.T) {
	const (
		p    = driver.OrientationPortrait
		pd   = driver.OrientationPortraitUpsideDown
		ll   = driver.OrientationLandscapeLeft
		lr   = driver.OrientationLandscapeRight
		land = driver.OrientationLandscape
		all  = p | pd | land
	)
	cases := []struct {
		name    string
		mask    driver.Orientation
		current driver.Orientation
		want    string
	}{
		{"portrait primary", p, 0, "portrait-primary"},
		{"portrait secondary", pd, 0, "portrait-secondary"},
		{"both portrait", p | pd, 0, "portrait"},
		{"landscape primary", ll, 0, "landscape-primary"},
		{"landscape secondary", lr, 0, "landscape-secondary"},
		{"both landscape", land, 0, "landscape"},
		{"all orientations", all, 0, "any"},
		{"mixed keeps current landscape right", p | lr, lr, "landscape-secondary"},
		{"mixed keeps current portrait", p | land, p, "portrait-primary"},
		{"mixed picks same axis portrait", pd | land, p, "portrait-secondary"},
		{"mixed picks same axis landscape", p | lr, ll, "landscape-secondary"},
		{"mixed fallback without current", pd | lr, 0, "portrait-secondary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := orientationLockType(tc.mask, tc.current); got != tc.want {
				t.Fatalf("orientationLockType(%v, %v) = %q, want %q", tc.mask, tc.current, got, tc.want)
			}
		})
	}
}

func TestParseOrientationValues(t *testing.T) {
	for typ, want := range map[string]driver.Orientation{
		"portrait-primary":    driver.OrientationPortrait,
		"portrait-secondary":  driver.OrientationPortraitUpsideDown,
		"landscape-primary":   driver.OrientationLandscapeLeft,
		"landscape-secondary": driver.OrientationLandscapeRight,
		"":                    0,
		"unknown":             0,
	} {
		if got := parseScreenOrientation(typ); got != want {
			t.Errorf("parseScreenOrientation(%q) = %v, want %v", typ, got, want)
		}
	}
	for angle, want := range map[int]driver.Orientation{
		0:    driver.OrientationPortrait,
		180:  driver.OrientationPortraitUpsideDown,
		-180: driver.OrientationPortraitUpsideDown,
		90:   driver.OrientationLandscapeLeft,
		-90:  driver.OrientationLandscapeRight,
		270:  driver.OrientationLandscapeRight,
		45:   0,
	} {
		if got := parseLegacyOrientation(angle); got != want {
			t.Errorf("parseLegacyOrientation(%d) = %v, want %v", angle, got, want)
		}
	}
}

func TestClassifyRefusal(t *testing.T) {
	cases := []struct {
		err      *screenError
		wantCode int
		wantMsg  string
	}{
		{&screenError{Name: "NotSupportedError", Message: "screen.orientation.lock() is not available on this device."}, driver.RefusalUnavailable, "screen.orientation.lock() is not available on this device."},
		{&screenError{Name: "SecurityError", Message: "The document must be in fullscreen mode."}, driver.RefusalDenied, "The document must be in fullscreen mode."},
		{&screenError{Name: "NotAllowedError", Message: "Permission denied"}, driver.RefusalDenied, "Permission denied"},
		{&screenError{Name: "TypeError", Message: "API can only be initiated by a user gesture."}, driver.RefusalDenied, "API can only be initiated by a user gesture."},
		{&screenError{Name: "TypeError", Message: "Disallowed by permissions policy"}, driver.RefusalDenied, "Disallowed by permissions policy"},
		{&screenError{Name: "TypeError", Message: "Document not active"}, driver.RefusalOther, "Document not active"},
		{&screenError{Name: "AbortError", Message: "Aborted"}, driver.RefusalOther, "Aborted"},
		{&screenError{Name: "NotSupportedError"}, driver.RefusalUnavailable, "NotSupportedError"},
		{nil, driver.RefusalOther, "fallback"},
	}
	for _, tc := range cases {
		code, msg := classifyRefusal(tc.err, "fallback")
		if code != tc.wantCode || msg != tc.wantMsg {
			t.Errorf("classifyRefusal(%+v) = (%d, %q), want (%d, %q)", tc.err, code, msg, tc.wantCode, tc.wantMsg)
		}
	}
}

func TestChromeAndroidTapEntersFullScreenAndLocksLandscape(t *testing.T) {
	fake := &fakeBrowserScreen{
		fullScreenEnabled:               true,
		fullScreenMethod:                true,
		requireGesture:                  true,
		orientationSupported:            true,
		requireFullScreenForOrientation: true,
		orientation:                     driver.OrientationPortrait,
		deferFullScreen:                 true,
	}
	ctrl := newScreenController(fake)

	// Simulate a single tap: user gesture active, SetFullScreen(true) + RequestOrientation(Landscape) in one commit.
	fake.userGesture = true
	ctrl.setFullScreen(true)
	ctrl.setOrientation(driver.OrientationLandscape)
	fake.userGesture = false

	// Orientation lock must wait for requestFullscreen to resolve so Chrome Android does not reject it with SecurityError.
	if len(fake.calls) != 1 || fake.calls[0] != "requestFullscreen" {
		t.Fatalf("calls before fullscreen settles = %v, want [requestFullscreen]", fake.calls)
	}
	if len(fake.events) != 0 {
		t.Fatalf("unexpected early errors: %+v", fake.events)
	}

	// Resolve requestFullscreen promise.
	fake.pendingFS(nil)
	if !fake.fullScreen || fake.locked != "landscape" || fake.orientation != driver.OrientationLandscapeLeft {
		t.Fatalf("state after fullscreen resolve: fs=%v locked=%q orient=%v", fake.fullScreen, fake.locked, fake.orientation)
	}
	if len(fake.events) != 0 {
		t.Fatalf("unexpected errors after resolve: %+v", fake.events)
	}
	if len(fake.calls) != 2 || fake.calls[1] != "orientation.lock(landscape)" {
		t.Fatalf("calls = %v", fake.calls)
	}

	// Releasing the orientation scope unlocks screen.orientation.
	ctrl.setOrientation(0)
	if fake.unlocks != 1 || fake.locked != "" {
		t.Fatalf("unlocks=%d locked=%q", fake.unlocks, fake.locked)
	}
	// A redundant unlock when already unlocked is a no-op.
	ctrl.setOrientation(0)
	if fake.unlocks != 1 {
		t.Fatalf("redundant unlock called unlock again: unlocks=%d", fake.unlocks)
	}
}

func TestFullScreenOutsideGestureAndOrientationWithoutFullScreenReportDenied(t *testing.T) {
	fake := &fakeBrowserScreen{
		fullScreenEnabled:               true,
		fullScreenMethod:                true,
		requireGesture:                  true,
		orientationSupported:            true,
		requireFullScreenForOrientation: true,
		orientation:                     driver.OrientationPortrait,
	}
	ctrl := newScreenController(fake)

	// Request fullscreen outside a user gesture -> TypeError classified as RefusalDenied.
	ctrl.setFullScreen(true)
	if len(fake.events) != 1 || fake.events[0].Kind != driver.EventFullScreenError || fake.events[0].Index != driver.RefusalDenied {
		t.Fatalf("fullscreen refusal = %+v", fake.events)
	}

	// Request orientation lock without fullscreen on Chrome Android -> SecurityError classified as RefusalDenied.
	ctrl.setOrientation(driver.OrientationLandscape)
	if len(fake.events) != 2 || fake.events[1].Kind != driver.EventOrientationError || fake.events[1].Index != driver.RefusalDenied {
		t.Fatalf("orientation refusal = %+v", fake.events)
	}
}

func TestSafariRefusalsReportedCleanly(t *testing.T) {
	t.Run("iOS Safari lacks fullscreen and orientation lock", func(t *testing.T) {
		fake := &fakeBrowserScreen{
			fullScreenEnabled:    false,
			fullScreenMethod:     false,
			orientationSupported: false,
			orientation:          driver.OrientationPortrait,
		}
		ctrl := newScreenController(fake)
		if fake.canFullScreen() || fake.canLockOrientation() {
			t.Fatalf("capabilities = (fs:%v, orient:%v), want (false, false)", fake.canFullScreen(), fake.canLockOrientation())
		}
		ctrl.setFullScreen(true)
		ctrl.setOrientation(driver.OrientationLandscape)
		if len(fake.events) != 2 {
			t.Fatalf("events = %+v, want 2 refusals", fake.events)
		}
		if fake.events[0].Kind != driver.EventFullScreenError || fake.events[0].Index != driver.RefusalUnavailable {
			t.Fatalf("fullscreen refusal = %+v, want RefusalUnavailable", fake.events[0])
		}
		if fake.events[1].Kind != driver.EventOrientationError || fake.events[1].Index != driver.RefusalUnavailable {
			t.Fatalf("orientation refusal = %+v, want RefusalUnavailable", fake.events[1])
		}
	})

	t.Run("macOS/iPad Safari allows fullscreen but rejects orientation lock with NotSupportedError", func(t *testing.T) {
		fake := &fakeBrowserScreen{
			fullScreenEnabled:    true,
			fullScreenMethod:     true,
			requireGesture:       true,
			orientationSupported: true,
			orientationLockErr:   &screenError{Name: "NotSupportedError", Message: "screen.orientation.lock() is not available on this device."},
			deferFullScreen:      true,
		}
		ctrl := newScreenController(fake)
		fake.userGesture = true
		ctrl.setFullScreen(true)
		ctrl.setOrientation(driver.OrientationLandscape)
		fake.userGesture = false
		fake.pendingFS(nil)
		if !fake.fullScreen {
			t.Fatal("fullscreen did not enter")
		}
		if len(fake.events) != 1 || fake.events[0].Kind != driver.EventOrientationError || fake.events[0].Index != driver.RefusalUnavailable {
			t.Fatalf("Safari orientation refusal = %+v, want single RefusalUnavailable", fake.events)
		}
	})
}

func TestLegacyWebKitAndDuplicateFullScreenErrorSuppression(t *testing.T) {
	t.Run("promise rejection suppresses duplicate fullscreenerror event", func(t *testing.T) {
		fake := &fakeBrowserScreen{
			fullScreenEnabled: true,
			fullScreenMethod:  true,
			requireGesture:    true,
			deferFullScreen:   true,
		}
		ctrl := newScreenController(fake)
		ctrl.setFullScreen(true) // outside gesture -> will reject
		// Browser fires fullscreenerror before/alongside promise rejection.
		ctrl.onFullScreenErrorEvent(nil)
		if len(fake.events) != 0 {
			t.Fatalf("fullscreenerror fired while promise pending: %+v", fake.events)
		}
		fake.pendingFS(nil)
		if len(fake.events) != 1 || fake.events[0].Kind != driver.EventFullScreenError || fake.events[0].Index != driver.RefusalDenied {
			t.Fatalf("events after rejection = %+v, want 1 denied event", fake.events)
		}
	})

	t.Run("legacy webkitRequestFullscreen reports webkitfullscreenerror and honors deferred orientation", func(t *testing.T) {
		fake := &fakeBrowserScreen{
			fullScreenEnabled:    true,
			fullScreenMethod:     true,
			legacyWebKit:         true,
			orientationSupported: true,
		}
		ctrl := newScreenController(fake)
		ctrl.setFullScreen(true)
		ctrl.setOrientation(driver.OrientationLandscape)
		// Legacy webkit succeeded via webkitfullscreenchange.
		fake.fullScreen = true
		ctrl.onFullScreenChange()
		if fake.locked != "landscape" {
			t.Fatalf("deferred orientation not locked after webkitfullscreenchange: locked=%q", fake.locked)
		}

		// Now test legacy webkitfullscreenerror.
		fake.fullScreen = false
		ctrl.setFullScreen(true)
		ctrl.onFullScreenErrorEvent(nil)
		if len(fake.events) != 1 || fake.events[0].Kind != driver.EventFullScreenError || fake.events[0].Index != driver.RefusalDenied {
			t.Fatalf("legacy webkitfullscreenerror = %+v", fake.events)
		}
	})
}

func TestSupersededOrientationLockIgnoresStaleAbortError(t *testing.T) {
	fake := &fakeBrowserScreen{
		fullScreenEnabled:    true,
		fullScreenMethod:     true,
		fullScreen:           true,
		orientationSupported: true,
		deferOrientation:     true,
	}
	ctrl := newScreenController(fake)
	ctrl.setOrientation(driver.OrientationLandscape)
	// Scope disposes before lock promise settles -> browser aborts the first promise.
	ctrl.setOrientation(0)
	if len(fake.pendingOrient) != 1 {
		t.Fatalf("pending orientation promises = %d, want 1", len(fake.pendingOrient))
	}
	fake.pendingOrient[0](&screenError{Name: "AbortError", Message: "The operation was aborted."})
	if len(fake.events) != 0 {
		t.Fatalf("stale AbortError should not be reported: %+v", fake.events)
	}
}

func TestReverseFullScreenDuringTransitionSkipsDeferredOrientation(t *testing.T) {
	fake := &fakeBrowserScreen{
		fullScreenEnabled:               true,
		fullScreenMethod:                true,
		requireGesture:                  true,
		orientationSupported:            true,
		requireFullScreenForOrientation: true,
		deferFullScreen:                 true,
	}
	ctrl := newScreenController(fake)
	fake.userGesture = true
	ctrl.setFullScreen(true)
	ctrl.setOrientation(driver.OrientationLandscape)
	ctrl.setFullScreen(false)
	fake.pendingFS(nil) // resolves entry -> triggers reverse exit
	fake.pendingFS(nil) // resolves exit
	if fake.fullScreen || fake.locked != "" {
		t.Fatalf("after reversal: fs=%v locked=%q", fake.fullScreen, fake.locked)
	}
}

// fakeWebPlatform wires screenController into a real engine.Engine so tests
// exercise core, desktop, Env signals, MountBatch ordering and event dispatch end to end.
type fakeWebPlatform struct {
	core.ApproxTextEngine
	fake   *fakeBrowserScreen
	ctrl   *screenController
	engine *engine.Engine
}

func (p *fakeWebPlatform) RequestFrame() {}
func (p *fakeWebPlatform) Mount(b *driver.MountBatch) {
	for i := range b.Items {
		it := &b.Items[i]
		switch it.Kind {
		case driver.ItemWindow:
			switch it.Window.Action {
			case driver.WindowSetFullScreen:
				p.ctrl.setFullScreen(it.Window.On)
			case driver.WindowToggleFullScreen:
				p.ctrl.setFullScreen(!p.fake.isFullScreen())
			}
		case driver.ItemOrientation:
			p.ctrl.setOrientation(driver.Orientation(it.Count))
		}
	}
}

func TestEndToEndWebFullScreenAndOrientationWithEngine(t *testing.T) {
	fake := &fakeBrowserScreen{
		fullScreenEnabled:               true,
		fullScreenMethod:                true,
		requireGesture:                  true,
		orientationSupported:            true,
		requireFullScreenForOrientation: true,
		orientation:                     driver.OrientationPortrait,
		deferFullScreen:                 true,
	}
	plat := &fakeWebPlatform{fake: fake}
	plat.ctrl = newScreenController(fake)

	var (
		root      *core.Scope
		playScope *core.Scope
		fsErrors  []error
		orErrors  []error
		env       *core.Env
		canFS     bool
		canOrient bool
		fsState   bool
		curOrient core.Orientation
	)
	e := engine.Start(plat, func(cx *core.Scope) core.Node {
		root = cx
		env = cx.Env()
		desktop.OnFullScreenError(cx, func(err error) { fsErrors = append(fsErrors, err) })
		cx.OnOrientationError(func(err error) { orErrors = append(orErrors, err) })
		cx.Effect(func() {
			canFS = desktop.CanFullScreen(cx)
			canOrient = env.CanLockOrientation()
			fsState = desktop.FullScreen(cx)
			curOrient = env.Orientation()
		})
		return core.View{}
	}, engine.Options{Size: driver.Size{Width: 390, Height: 844}, Environment: fake.environment()})
	defer e.Close()
	plat.engine = e
	fake.onEnv = func(ev driver.Environment) { e.SetEnvironment(ev) }
	fake.onDispatch = func(ev driver.Event) { e.Dispatch(ev) }
	e.Sync()

	if !canFS || !canOrient || fsState || curOrient != core.OrientationPortrait {
		t.Fatalf("initial env: canFS=%v canOrient=%v fs=%v orient=%v", canFS, canOrient, fsState, curOrient)
	}

	// 1. Calling SetFullScreen outside a user gesture reports CodeDenied.
	root.Post(func() { desktop.SetFullScreen(root, true) })
	e.Sync()
	if fake.pendingFS == nil {
		t.Fatal("expected pending requestFullscreen promise")
	}
	fake.pendingFS(nil)
	e.Sync()
	if len(fsErrors) != 1 || core.ErrorCode(fsErrors[0]) != core.CodeDenied {
		t.Fatalf("fsErrors = %+v, want 1 CodeDenied", fsErrors)
	}

	// 2. Tap enters fullscreen and locks landscape together.
	fake.userGesture = true
	root.Post(func() {
		desktop.SetFullScreen(root, true)
		playScope = root.Child()
		playScope.Run(func() { playScope.RequestOrientation(core.OrientationLandscape) })
	})
	e.Sync()
	fake.userGesture = false
	fake.pendingFS(nil)
	e.Sync()

	if !fsState || curOrient != core.OrientationLandscapeLeft || len(orErrors) != 0 {
		t.Fatalf("after tap: fs=%v orient=%v orErrors=%+v", fsState, curOrient, orErrors)
	}

	// 3. Browser fires screen.orientation change event -> Env.Orientation updates reactively.
	fake.orientation = driver.OrientationLandscapeRight
	plat.ctrl.onOrientationChange()
	e.Sync()
	if curOrient != core.OrientationLandscapeRight {
		t.Fatalf("orientation after change event = %v, want LandscapeRight", curOrient)
	}

	// 4. Disposing playScope unlocks orientation; external fullscreen exit updates Env.FullScreen.
	root.Post(func() { playScope.Dispose() })
	e.Sync()
	if fake.unlocks != 1 {
		t.Fatalf("unlocks = %d, want 1", fake.unlocks)
	}
	fake.fullScreen = false
	plat.ctrl.onFullScreenChange()
	e.Sync()
	if fsState {
		t.Fatal("Env.FullScreen stayed true after external fullscreenchange")
	}

	// 5. Safari-like NotSupportedError on retry after prior refusal re-emits cleanly.
	fake.orientationLockErr = &screenError{Name: "NotSupportedError", Message: "screen.orientation.lock() is not available on this device."}
	root.Post(func() {
		s1 := root.Child()
		s1.Run(func() { s1.RequestOrientation(core.OrientationLandscape) })
	})
	e.Sync()
	if len(orErrors) != 1 || core.ErrorCode(orErrors[0]) != core.CodeUnavailable {
		t.Fatalf("orErrors = %+v, want 1 CodeUnavailable", orErrors)
	}
	// Retrying the same mask after a refusal must emit again rather than being suppressed by sent == mask.
	root.Post(func() {
		s2 := root.Child()
		s2.Run(func() { s2.RequestOrientation(core.OrientationLandscape) })
	})
	e.Sync()
	if len(orErrors) != 2 || core.ErrorCode(orErrors[1]) != core.CodeUnavailable {
		t.Fatalf("retry orErrors = %+v, want 2 CodeUnavailable", orErrors)
	}
}
