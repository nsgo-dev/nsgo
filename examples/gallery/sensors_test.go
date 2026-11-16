package main

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestSystemSensors(t *testing.T) {
	headless(t)
	app := startDesktop(t)
	app.Permissions[core.PermissionMotion] = core.PermissionGranted
	open(app, "System")
	app.Press("id:sensor-locate")
	if got := app.Find("id:sensor-location").Text; !strings.Contains(got, string(core.SensorNotAllowed)) {
		t.Fatalf("a fix without the permission showed %q", got)
	}
	if app.Permissions[core.PermissionLocation] != core.PermissionNotAsked {
		t.Fatal("locating asked for the permission")
	}
	app.Press("id:request-" + string(core.PermissionLocation))
	if app.Permissions[core.PermissionLocation] != core.PermissionGranted {
		t.Fatal("the Permissions card did not ask for location")
	}
	app.Press("id:sensor-locate")
	app.FeedLocation(core.LocationFix{Latitude: 51.5, Longitude: -0.12, Timestamp: app.Clock.Now(), HorizontalAccuracy: core.SensorNumber{Value: 5, Valid: true}})
	app.Settle()
	if got := app.Find("id:sensor-location").Text; !strings.Contains(got, "51.50000, -0.12000") {
		t.Fatalf("location %q", got)
	}
	if app.ActiveSensorStreams() != 0 {
		t.Fatal("one-shot retained provider")
	}
	app.Press("id:sensor-track")
	app.Press("id:sensor-accelerometer")
	if app.ActiveSensorStreams() != 2 {
		t.Fatal("location and motion did not run independently")
	}
	app.FeedMotion(core.MotionSample{Sensor: core.SensorAccelerometer, Vector: core.SensorVector{Z: 9.81}})
	app.Settle()
	if got := app.Find("id:sensor-motion").Text; !strings.Contains(got, "9.81") {
		t.Fatalf("motion %q", got)
	}
	app.Press("id:sensor-stop")
	if app.ActiveSensorStreams() != 0 {
		t.Fatal("stop retained provider")
	}
	app.Press("id:sensor-track")
	open(app, "Button")
	if app.ActiveSensorStreams() != 0 {
		t.Fatal("leaving System retained provider")
	}
}
