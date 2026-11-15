package core

import (
	"fmt"
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// SensorKind identifies a provider's separately requested permission.
type SensorKind = driver.SensorKind

const (
	SensorLocation           = driver.SensorLocation
	SensorBackgroundLocation = driver.SensorBackgroundLocation
	SensorMotion             = driver.SensorMotion
)

// SensorAuthorization is a provider's current system authorization.
type SensorAuthorization = driver.SensorAuthorization

const (
	SensorNotAsked    = driver.SensorNotAsked
	SensorGranted     = driver.SensorGranted
	SensorDenied      = driver.SensorDenied
	SensorRestricted  = driver.SensorRestricted
	SensorUnavailable = driver.SensorUnavailable
)

// SensorGrant reports authorization and whether location is precise.
type SensorGrant = driver.SensorGrant

// SensorNumber is a measurement with an explicit availability flag.
type SensorNumber = driver.SensorNumber

// LocationFix is a WGS 84 fix, with optional measurements and provider times.
type LocationFix = driver.LocationFix

// LocationOptions chooses accuracy, distance, caching and background policy.
type LocationOptions = driver.LocationOptions

// MotionOptions chooses a sensor set and sample interval.
type MotionOptions = driver.MotionOptions

// MotionSensors is a set of raw and fused device sensors.
type MotionSensors = driver.MotionSensors

const (
	SensorAccelerometer = driver.SensorAccelerometer
	SensorGyroscope     = driver.SensorGyroscope
	SensorMagnetometer  = driver.SensorMagnetometer
	SensorDeviceMotion  = driver.SensorDeviceMotion
)

// SensorVector is a vector in the natural device frame, in SI units (µT for magnetism).
type SensorVector = driver.SensorVector

// SensorQuaternion rotates device axes into world axes.
type SensorQuaternion = driver.SensorQuaternion

// MotionSample is one sensor's latest sample and its dropped-sample count.
type MotionSample = driver.MotionSample

// SensorError is a stable provider failure; it never contains location data.
type SensorError string

const (
	SensorNotAllowed    SensorError = "not-allowed"
	SensorNotAvailable  SensorError = "unavailable"
	SensorTimedOut      SensorError = "timeout"
	SensorFailed        SensorError = "failed"
	SensorNotConfigured SensorError = "not-configured"
)

func (e SensorError) Error() string { return "nsgo: sensor: " + string(e) }

// SensorStream owns a pending location, permission request or live stream.
// Its callbacks run on the engine goroutine only while its scope is alive.
type SensorStream struct {
	tree                 *Tree
	owner                *Scope
	id                   uint32
	request              driver.SensorRequest
	location             func(LocationFix)
	motion               func(MotionSample)
	permission           func(SensorGrant)
	onError              func(error)
	stopped, paused, gap bool
	stopEffect           func()
	stopTimer            func() bool
	remaining            time.Duration
	started              time.Time
}

func sensorCapability(kind SensorKind) string {
	switch kind {
	case SensorLocation:
		return AccessLocation
	case SensorBackgroundLocation:
		return AccessBackgroundLocation
	case SensorMotion:
		return AccessMotion
	}
	return ""
}

// ReadSensorPermission reads the current grant without showing a prompt.
// Stop cancels the answer, as does disposal of the requesting scope.
func (cx *Scope) ReadSensorPermission(kind SensorKind, done func(SensorGrant)) (*SensorStream, error) {
	return cx.sensorPermission(kind, driver.SensorReadPermission, done)
}

// RequestSensorPermission explicitly asks the system for this grant.
// Android background permission is requested separately after foreground
// permission; on Android 11+ the system settings page supplies that grant.
// Starting a stream never requests permission implicitly.
func (cx *Scope) RequestSensorPermission(kind SensorKind, done func(SensorGrant)) (*SensorStream, error) {
	return cx.sensorPermission(kind, driver.SensorRequestPermission, done)
}

func (cx *Scope) sensorPermission(kind SensorKind, command driver.SensorCommand, done func(SensorGrant)) (*SensorStream, error) {
	if kind > SensorMotion {
		return nil, fmt.Errorf("nsgo: invalid sensor kind")
	}
	s, err := cx.newSensor(driver.SensorRequest{Kind: kind, Command: command})
	if err != nil {
		return nil, err
	}
	s.permission = done
	s.send()
	return s, nil
}

// Locate waits for a new provider fix unless MaxAge explicitly allows a
// cached one. It waits through background pauses and stops after one fix.
// The default foreground timeout is 30 seconds; Stop cancels silently.
func (cx *Scope) Locate(opts LocationOptions, done func(LocationFix, error)) (*SensorStream, error) {
	s, err := cx.locationSensor(opts, driver.SensorLocate)
	if err != nil {
		return nil, err
	}
	s.location = func(f LocationFix) {
		if done != nil {
			done(f, nil)
		}
	}
	s.onError = func(err error) {
		if done != nil {
			done(LocationFix{}, err)
		}
	}
	s.remaining = opts.Timeout
	if s.remaining == 0 {
		s.remaining = 30 * time.Second
	}
	s.followLifecycle(cx)
	return s, nil
}

// WatchLocation starts an explicitly authorized stream. Foreground streams
// pause in background and mark their first resumed fix with Gap. Background
// streams require AccessLocation and AccessBackgroundLocation plus the
// platform declaration and grant; they never restart a killed process.
func (cx *Scope) WatchLocation(opts LocationOptions, sample func(LocationFix), failed func(error)) (*SensorStream, error) {
	s, err := cx.locationSensor(opts, driver.SensorWatchLocation)
	if err != nil {
		return nil, err
	}
	s.location, s.onError = sample, failed
	s.followLifecycle(cx)
	return s, nil
}

func (cx *Scope) locationSensor(opts LocationOptions, command driver.SensorCommand) (*SensorStream, error) {
	if math.IsNaN(opts.Accuracy) || math.IsInf(opts.Accuracy, 0) || opts.Accuracy < 0 ||
		math.IsNaN(opts.Distance) || math.IsInf(opts.Distance, 0) || opts.Distance < 0 ||
		opts.Interval < 0 || opts.MaxAge < 0 || opts.Timeout < 0 {
		return nil, fmt.Errorf("nsgo: invalid location options")
	}
	if opts.Background {
		if err := cx.CheckAccess(AccessRequest{Permission: AccessBackgroundLocation}); err != nil {
			return nil, err
		}
	}
	return cx.newSensor(driver.SensorRequest{Kind: SensorLocation, Command: command, Location: opts})
}

// WatchMotion starts raw or fused sensors at the requested interval,
// clamped to 10 ms–1 s. Zero requests accelerometer at 20 ms. Samples use
// natural device axes, with +9.80665 m/s² on z face-up, and report dropped
// samples. Motion always pauses in background; it never keeps a process alive.
func (cx *Scope) WatchMotion(opts MotionOptions, sample func(MotionSample), failed func(error)) (*SensorStream, error) {
	if opts.Sensors == 0 {
		opts.Sensors = SensorAccelerometer
	}
	if opts.Sensors & ^(SensorAccelerometer|SensorGyroscope|SensorMagnetometer|SensorDeviceMotion) != 0 || opts.Interval < 0 {
		return nil, fmt.Errorf("nsgo: invalid motion options")
	}
	if opts.Interval == 0 {
		opts.Interval = 20 * time.Millisecond
	}
	opts.Interval = min(max(opts.Interval, 10*time.Millisecond), time.Second)
	s, err := cx.newSensor(driver.SensorRequest{Kind: SensorMotion, Command: driver.SensorWatchMotion, Motion: opts})
	if err != nil {
		return nil, err
	}
	s.motion, s.onError = sample, failed
	s.followLifecycle(cx)
	return s, nil
}

func (cx *Scope) newSensor(r driver.SensorRequest) (*SensorStream, error) {
	if err := cx.CheckAccess(AccessRequest{Permission: sensorCapability(r.Kind)}); err != nil {
		return nil, err
	}
	t := cx.tree()
	if t == nil || cx.current().disposed {
		return nil, SensorNotAvailable
	}
	t.alertID++
	s := &SensorStream{tree: t, owner: cx.current(), id: t.alertID, request: r}
	if t.sensors == nil {
		t.sensors = map[uint32]*SensorStream{}
	}
	t.sensors[s.id] = s
	s.owner.addCleanup(s.Stop)
	return s, nil
}

func (s *SensorStream) followLifecycle(cx *Scope) {
	s.paused = true
	s.stopEffect = cx.Effect(func() {
		pause := cx.Env().Lifecycle() == LifecycleBackground && !s.request.Location.Background
		if s.stopped || pause == s.paused {
			return
		}
		s.paused = pause
		if pause {
			s.gap = true
			if s.stopTimer != nil {
				s.stopTimer()
				s.remaining = max(0, s.remaining-s.owner.Now().Sub(s.started))
			}
			s.command(driver.SensorStop)
		} else {
			s.request.Generation++
			s.send()
			if s.request.Command == driver.SensorLocate {
				s.tree.within(s.owner, s.armTimeout)
			}
		}
	})
}

func (s *SensorStream) armTimeout() {
	s.started = s.owner.Now()
	s.stopTimer = s.owner.AfterFunc(s.remaining, func() {
		if !s.stopped && !s.paused {
			s.fail(SensorTimedOut)
		}
	})
}

func (s *SensorStream) send() { s.command(s.request.Command) }

func (s *SensorStream) command(command driver.SensorCommand) {
	r := s.request
	r.Command = command
	s.tree.commands = append(s.tree.commands, driver.MountItem{Kind: driver.ItemSensor, Count: s.id, Sensor: &r})
	s.tree.changed = true
}

// Stop cancels this request and its provider; queued answers are ignored.
func (s *SensorStream) Stop() {
	if s == nil || s.stopped {
		return
	}
	s.stopped = true
	delete(s.tree.sensors, s.id)
	if s.stopEffect != nil {
		s.stopEffect()
	}
	if s.stopTimer != nil {
		s.stopTimer()
	}
	s.command(driver.SensorStop)
}

// Stopped reports cancellation, completion or a terminal provider failure.
func (s *SensorStream) Stopped() bool { return s == nil || s.stopped }

func (s *SensorStream) fail(err error) {
	s.Stop()
	if !s.owner.disposed && s.onError != nil {
		s.tree.within(s.owner, func() { s.onError(err) })
	}
}

func (t *Tree) sensorEvent(ev driver.Event) {
	var r driver.SensorReport
	switch v := ev.Data.(type) {
	case *driver.SensorReport:
		r = *v
	case *driver.SensorPacket:
		var ok bool
		r, ok = v.Take()
		if !ok {
			return
		}
	default:
		return
	}
	s := t.sensors[ev.Count]
	if s == nil || s.stopped || s.paused || s.owner.disposed || r.Generation != s.request.Generation {
		return
	}
	if r.Error != "" {
		s.fail(SensorError(r.Error))
		return
	}
	if r.Grant != nil {
		s.Stop()
		if s.permission != nil {
			t.within(s.owner, func() { s.permission(*r.Grant) })
		}
		return
	}
	if r.Location != nil && s.location != nil {
		f := *r.Location
		f.Gap, s.gap = f.Gap || s.gap, false
		if s.request.Command == driver.SensorLocate {
			s.Stop()
		}
		t.within(s.owner, func() { s.location(f) })
	}
	if r.Motion != nil && s.motion != nil {
		m := *r.Motion
		m.Gap, s.gap = m.Gap || s.gap, false
		t.within(s.owner, func() { s.motion(m) })
	}
	if r.End {
		s.Stop()
	}
}
