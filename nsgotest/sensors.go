package nsgotest

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

type sensorRun struct {
	request driver.SensorRequest
	packets map[driver.MotionSensors]*driver.SensorPacket
}

func (a *App) sensor(id uint32, r driver.SensorRequest) {
	if r.Command == driver.SensorStop {
		delete(a.sensorRuns, id)
		return
	}
	report := func(e driver.SensorReport) {
		e.Generation = r.Generation
		a.post(driver.Event{Kind: driver.EventSensor, Count: id, Data: &e})
	}
	grant, exists := a.SensorPermissions[r.Kind]
	if !exists {
		grant.Authorization = core.SensorUnavailable
	}
	if r.Command <= driver.SensorRequestPermission {
		if r.Command == driver.SensorRequestPermission {
			a.SensorPrompts = append(a.SensorPrompts, r.Kind)
			if answer, ok := a.SensorAnswers[r.Kind]; ok {
				grant = answer
				if a.SensorPermissions == nil {
					a.SensorPermissions = map[core.SensorKind]core.SensorGrant{}
				}
				a.SensorPermissions[r.Kind] = grant
			}
		}
		report(driver.SensorReport{Grant: &grant})
		return
	}
	if grant.Authorization != core.SensorGranted {
		code := "not-allowed"
		if grant.Authorization == core.SensorUnavailable {
			code = "unavailable"
		}
		report(driver.SensorReport{Error: code, End: true})
		return
	}
	if r.Location.Background && a.SensorPermissions[core.SensorBackgroundLocation].Authorization != core.SensorGranted {
		report(driver.SensorReport{Error: "not-allowed", End: true})
		return
	}
	if r.Command == driver.SensorLocate && r.Location.MaxAge > 0 && a.LocationCache != nil {
		f := *a.LocationCache
		f.Age = a.Clock.Now().Sub(f.Timestamp)
		if f.Age >= 0 && f.Age <= r.Location.MaxAge {
			f.Cached = true
			report(driver.SensorReport{Location: &f, End: true})
			return
		}
	}
	if a.sensorRuns == nil {
		a.sensorRuns = map[uint32]*sensorRun{}
	}
	a.sensorRuns[id] = &sensorRun{request: r, packets: map[driver.MotionSensors]*driver.SensorPacket{}}
}

func (a *App) sensorSample(id uint32, run *sensorRun, key driver.MotionSensors, r driver.SensorReport) {
	p := run.packets[key]
	if p == nil {
		p = &driver.SensorPacket{}
		run.packets[key] = p
	}
	r.Generation = run.request.Generation
	if p.Offer(r) {
		a.post(driver.Event{Kind: driver.EventSensor, Count: id, Data: p})
	}
}

// FeedLocation delivers a copied fix to active location requests. Call
// several times before Settle to test latest-value back-pressure.
func (a *App) FeedLocation(f core.LocationFix) {
	a.requireSensorModel()
	for id, run := range a.sensorRuns {
		if run.request.Kind == driver.SensorLocation {
			copy := f
			a.sensorSample(id, run, 0, driver.SensorReport{Location: &copy})
		}
	}
}

// FeedMotion delivers a copied reading to streams requesting that sensor.
func (a *App) FeedMotion(m core.MotionSample) {
	a.requireSensorModel()
	for id, run := range a.sensorRuns {
		if run.request.Kind == driver.SensorMotion && run.request.Motion.Sensors&m.Sensor != 0 {
			copy := m
			copy.Interval = run.request.Motion.Interval
			a.sensorSample(id, run, m.Sensor, driver.SensorReport{Motion: &copy})
		}
	}
}

// ActiveSensorStreams is the number of native providers the model holds.
func (a *App) ActiveSensorStreams() int { a.requireSensorModel(); return len(a.sensorRuns) }

func (a *App) requireSensorModel() {
	a.t.Helper()
	if a.target != nil {
		a.t.Skip("nsgotest: synthetic sensors run headless only")
	}
}
