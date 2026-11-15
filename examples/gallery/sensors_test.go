package main

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestSystemSensors(t *testing.T) {
	headless(t)
	app := startDesktop(t)
	app.SensorPermissions = map[core.SensorKind]core.SensorGrant{
		core.SensorLocation: {Authorization: core.SensorNotAsked}, core.SensorMotion: {Authorization: core.SensorGranted},
	}
	app.SensorAnswers = map[core.SensorKind]core.SensorGrant{core.SensorLocation: {Authorization: core.SensorGranted, Precise: true}}
	open(app, "System")
	app.Press("id:sensor-location-ask")
	if len(app.SensorPrompts) != 1 {
		t.Fatal("location permission did not use explicit prompt")
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
