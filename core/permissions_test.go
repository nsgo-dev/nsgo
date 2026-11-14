package core_test

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestPermissionsCheckAskAndOpenSettings(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	var got []core.PermissionStatus
	answer := func(s core.PermissionStatus) { got = append(got, s) }
	app.PermissionAnswers[core.PermissionMicrophone] = core.PermissionDenied
	scope.PermissionStatus(core.PermissionCamera, answer)
	scope.RequestPermission(core.PermissionCamera, answer)
	scope.RequestPermission(core.PermissionCamera, answer)
	scope.RequestPermission(core.PermissionMicrophone, answer)
	scope.PermissionStatus(core.PermissionMicrophone, answer)
	app.Settle()
	want := []core.PermissionStatus{core.PermissionNotAsked, core.PermissionGranted, core.PermissionGranted, core.PermissionDenied, core.PermissionDenied}
	if !slices.Equal(got, want) {
		t.Errorf("statuses %v, want %v", got, want)
	}
	var opened []bool
	scope.OpenPermissionSettings(core.PermissionMicrophone, func(ok bool) { opened = append(opened, ok) })
	app.Settle()
	if !slices.Equal(opened, []bool{true}) || !slices.Equal(app.PermissionSettings, []core.Permission{core.PermissionMicrophone}) {
		t.Errorf("opened %v, settings %v", opened, app.PermissionSettings)
	}
}

func TestPermissionUnknownKeyAndAccessRefusal(t *testing.T) {
	var got []core.PermissionStatus
	answer := func(s core.PermissionStatus) { got = append(got, s) }
	var opened []bool
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		// Answered inline, sending nothing.
		cx.RequestPermission("nope", answer)
		cx.OpenPermissionSettings("nope", func(ok bool) { opened = append(opened, ok) })
		cx.ProvideAccess(refuseKey{core.PermissionCamera})
		cx.RequestPermission(core.PermissionCamera, answer)
		cx.OpenPermissionSettings(core.PermissionCamera, func(ok bool) { opened = append(opened, ok) })
		cx.RequestPermission(core.PermissionMicrophone, answer)
		cx.GuidePermission(core.PermissionCamera, core.PermissionGuideOptions{}, answer)
		// Code under an Access names no other app for the user to give a permission to.
		cx.GuidePermission(core.PermissionMicrophone, core.PermissionGuideOptions{App: "/Applications/Other.app"}, answer)
		return core.View{}
	})
	app.Settle()
	want := []core.PermissionStatus{core.PermissionUnavailable, core.PermissionUnavailable, core.PermissionUnavailable, core.PermissionUnavailable, core.PermissionGranted}
	if !slices.Equal(got, want) {
		t.Errorf("statuses %v, want %v", got, want)
	}
	if !slices.Equal(opened, []bool{false, false}) || len(app.PermissionSettings) != 0 || app.Guide != "" {
		t.Errorf("opened %v, settings %v, guide %q", opened, app.PermissionSettings, app.Guide)
	}
	if s := app.Permissions[core.PermissionCamera]; s != core.PermissionNotAsked {
		t.Errorf("the refused camera reached the platform: %v", s)
	}
}

type refuseKey struct{ p core.Permission }

func (r refuseKey) Allows(req core.AccessRequest) bool {
	return req.Permission != core.AccessPermissions || req.Resource != string(r.p)
}
func (refuseKey) WebHosts() []string { return nil }

func TestPermissionGuideEndsOnceForEachWay(t *testing.T) {
	var scope *core.Scope
	var shown *core.Signal[bool]
	ends := map[string][]core.PermissionStatus{}
	ended := func(name string) func(core.PermissionStatus) {
		return func(s core.PermissionStatus) { ends[name] = append(ends[name], s) }
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		shown = core.NewSignal(cx, false)
		return core.Show{When: shown.Get, Then: func(cx *core.Scope) core.Node {
			cx.GuidePermission(core.PermissionFullDiskAccess, core.PermissionGuideOptions{}, ended("owner"))
			return core.View{}
		}}
	})
	check := func(name string, want ...core.PermissionStatus) {
		t.Helper()
		if !slices.Equal(ends[name], want) {
			t.Errorf("%s ended %v, want %v", name, ends[name], want)
		}
	}

	// Granted in the settings.
	scope.GuidePermission(core.PermissionAccessibility, core.PermissionGuideOptions{}, ended("granted"))
	app.Settle()
	if app.Guide != core.PermissionAccessibility {
		t.Fatalf("guide %q", app.Guide)
	}
	app.GrantPermission(core.PermissionAccessibility)
	check("granted", core.PermissionGranted)
	if app.Guide != "" {
		t.Errorf("guide %q after the grant", app.Guide)
	}

	// A guide of what the app has ends at once.
	scope.GuidePermission(core.PermissionAccessibility, core.PermissionGuideOptions{}, ended("had"))
	app.Settle()
	check("had", core.PermissionGranted)

	// A new guide ends the one before, and stop ends it once.
	scope.GuidePermission(core.PermissionScreenRecording, core.PermissionGuideOptions{}, ended("first"))
	stop := scope.GuidePermission(core.PermissionInputMonitoring, core.PermissionGuideOptions{}, ended("second"))
	app.Settle()
	check("first", core.PermissionNotAsked)
	if app.Guide != core.PermissionInputMonitoring {
		t.Errorf("guide %q", app.Guide)
	}
	stop()
	stop()
	app.Settle()
	check("second", core.PermissionNotAsked)

	// The user closes it.
	scope.GuidePermission(core.PermissionBluetooth, core.PermissionGuideOptions{}, ended("closed"))
	app.Settle()
	app.CloseGuide()
	check("closed", core.PermissionNotAsked)

	// Its owner goes, and done runs under the root.
	shown.Set(true)
	app.Settle()
	if app.Guide != core.PermissionFullDiskAccess {
		t.Fatalf("guide %q", app.Guide)
	}
	shown.Set(false)
	app.Settle()
	check("owner", core.PermissionNotAsked)
	if app.Guide != "" {
		t.Errorf("guide %q after its owner went", app.Guide)
	}
	check("granted", core.PermissionGranted)
	check("first", core.PermissionNotAsked)
}

func TestPermissionStatusStrings(t *testing.T) {
	for _, c := range []struct {
		s       core.PermissionStatus
		name    string
		allowed bool
	}{
		{core.PermissionNotAsked, "prompt", false},
		{core.PermissionGranted, "granted", true},
		{core.PermissionDenied, "denied", false},
		{core.PermissionUnknown, "unknown", false},
		{core.PermissionNotGated, "not-gated", true},
		{core.PermissionUnavailable, "unavailable", false},
	} {
		if c.s.String() != c.name || c.s.Allowed() != c.allowed {
			t.Errorf("%d: %q allowed %v, want %q %v", c.s, c.s.String(), c.s.Allowed(), c.name, c.allowed)
		}
	}
	if len(core.Permissions()) != 12 {
		t.Errorf("%d permissions", len(core.Permissions()))
	}
}
