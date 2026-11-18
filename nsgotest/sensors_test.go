package nsgotest

import (
	"errors"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

func TestSensorModelPermissionsCachingAndDisposal(t *testing.T) {
	var cx *core.Scope
	app := Start(t, func(c *core.Scope) core.Node { cx = c; return core.View{} })
	app.requireSensorModel()
	app.PermissionAnswers[core.PermissionLocation] = core.PermissionGranted
	var status core.PermissionStatus
	app.Post(func() { cx.PermissionStatus(core.PermissionLocation, func(s core.PermissionStatus) { status = s }) })
	app.Settle()
	if status != core.PermissionNotAsked || app.Permissions[core.PermissionLocation] != core.PermissionNotAsked {
		t.Fatal("checking asked")
	}
	var refused error
	app.Post(func() {
		_, _ = cx.Locate(core.LocationOptions{}, func(_ core.LocationFix, err error) { refused = err })
	})
	app.Settle()
	if !errors.Is(refused, core.SensorNotAllowed) || app.Permissions[core.PermissionLocation] != core.PermissionNotAsked {
		t.Fatalf("locating without the permission: %v, and it asked: %v", refused, app.Permissions[core.PermissionLocation])
	}
	app.Post(func() { cx.RequestPermission(core.PermissionLocation, func(s core.PermissionStatus) { status = s }) })
	app.Settle()
	if status != core.PermissionGranted || app.Permissions[core.PermissionLocation] != core.PermissionGranted {
		t.Fatalf("asking answered %v", status)
	}
	app.LocationCache = &core.LocationFix{Latitude: 5, Timestamp: app.Clock.Now().Add(-time.Minute)}
	var fixes []core.LocationFix
	var s *core.SensorStream
	app.Post(func() {
		s, _ = cx.Locate(core.LocationOptions{}, func(f core.LocationFix, err error) {
			if err != nil {
				t.Fatal(err)
			}
			fixes = append(fixes, f)
		})
	})
	app.Settle()
	if len(fixes) != 0 || s.Stopped() || app.ActiveSensorStreams() != 1 {
		t.Fatal("default one-shot used cache")
	}
	app.FeedLocation(core.LocationFix{Latitude: 6, Timestamp: app.Clock.Now()})
	app.Settle()
	if len(fixes) != 1 || fixes[0].Latitude != 6 || !s.Stopped() || app.ActiveSensorStreams() != 0 {
		t.Fatalf("fresh fix %+v", fixes)
	}
	app.Post(func() {
		_, _ = cx.Locate(core.LocationOptions{MaxAge: 2 * time.Minute}, func(f core.LocationFix, _ error) { fixes = append(fixes, f) })
	})
	app.Settle()
	if len(fixes) != 2 || !fixes[1].Cached || fixes[1].Age != time.Minute {
		t.Fatalf("cache %+v", fixes)
	}
	app.Permissions[core.PermissionMotion] = core.PermissionGranted
	var samples []core.MotionSample
	child := cx.Child()
	app.Post(func() {
		_, _ = child.WatchMotion(core.MotionOptions{Interval: time.Nanosecond}, func(m core.MotionSample) { samples = append(samples, m) }, nil)
	})
	app.Settle()
	for i := 0; i < 100; i++ {
		app.FeedMotion(core.MotionSample{Sensor: core.SensorAccelerometer, Vector: core.SensorVector{Z: float64(i)}})
	}
	app.Settle()
	if len(samples) != 1 || samples[0].Vector.Z != 99 || samples[0].Dropped != 99 || samples[0].Interval != 10*time.Millisecond {
		t.Fatalf("samples %+v", samples)
	}
	app.FeedMotion(core.MotionSample{Sensor: core.SensorAccelerometer})
	app.Post(child.Dispose)
	app.Settle()
	if app.ActiveSensorStreams() != 0 {
		t.Fatal("disposed scope retained provider")
	}
	app.FeedMotion(core.MotionSample{Sensor: core.SensorAccelerometer})
	app.Settle()
	if len(samples) > 2 {
		t.Fatal("disposed scope heard sample")
	}
}
