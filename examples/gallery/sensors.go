package main

import (
	"fmt"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

func sensorsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	location := core.NewSignal(cx, tr.SystemSensorsIdle())
	motion := core.NewSignal(cx, tr.SystemSensorsIdle())
	var tracking, moving *core.SensorStream
	fail := func(err error) {
		if err != nil {
			location.Set(err.Error())
		}
	}
	fix := func(f core.LocationFix) {
		location.Set(fmt.Sprintf("%.5f, %.5f · ±%.0f m · %d", f.Latitude, f.Longitude, f.HorizontalAccuracy.Value, f.Dropped))
	}
	button := func(text, id string, fn func()) core.Node {
		return ui.Button{Text: text, TestID: id, Size: ui.SizeSmall, Variant: ui.ButtonSecondary, OnPress: fn}
	}
	startLocation := func(background bool) {
		tracking.Stop()
		var err error
		tracking, err = cx.WatchLocation(core.LocationOptions{Accuracy: 10, Interval: time.Second, Background: background,
			NotificationTitle: tr.SystemSensorsTitle(), NotificationBody: tr.SystemSensorsTracking()}, fix, fail)
		fail(err)
	}
	return Card{Title: tr.SystemSensorsTitle(), Description: tr.SystemSensorsSummary(), Children: []core.Node{
		ui.Wrap{Children: []core.Node{
			button(tr.SystemSensorsLocate(), "sensor-locate", func() {
				_, err := cx.Locate(core.LocationOptions{}, func(f core.LocationFix, err error) {
					if err != nil {
						fail(err)
					} else {
						fix(f)
					}
				})
				fail(err)
			}),
			button(tr.SystemSensorsTrack(), "sensor-track", func() { startLocation(false) }),
			button(tr.SystemSensorsBackground(), "sensor-background", func() { startLocation(true) }),
			button(tr.SystemSensorsStop(), "sensor-stop", func() { tracking.Stop(); moving.Stop() }),
		}},
		core.Text{TestID: "sensor-location", ContentFunc: location.Get, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
		ui.Wrap{Children: []core.Node{
			button(tr.SystemSensorsAccelerometer(), "sensor-accelerometer", func() {
				moving.Stop()
				var err error
				moving, err = cx.WatchMotion(core.MotionOptions{}, func(m core.MotionSample) {
					motion.Set(fmt.Sprintf("%.2f, %.2f, %.2f · %d", m.Vector.X, m.Vector.Y, m.Vector.Z, m.Dropped))
				}, func(err error) { motion.Set(err.Error()) })
				fail(err)
			}),
			button(tr.SystemSensorsFused(), "sensor-fused", func() {
				moving.Stop()
				var err error
				moving, err = cx.WatchMotion(core.MotionOptions{Sensors: core.SensorGyroscope | core.SensorMagnetometer | core.SensorDeviceMotion}, func(m core.MotionSample) {
					if m.Sensor == core.SensorDeviceMotion {
						motion.Set(fmt.Sprintf("q %.2f, %.2f, %.2f, %.2f · %d", m.Attitude.X, m.Attitude.Y, m.Attitude.Z, m.Attitude.W, m.Dropped))
					}
				}, func(err error) { motion.Set(err.Error()) })
				fail(err)
			}),
		}},
		core.Text{TestID: "sensor-motion", ContentFunc: motion.Get, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}
