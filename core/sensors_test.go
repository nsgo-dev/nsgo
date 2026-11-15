package core

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func sensorHost(t *testing.T) (*host, *Scope) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	t.Cleanup(h.root.Dispose)
	return h, cx
}

func sensorAnswer(h *host, it *driver.MountItem, report driver.SensorReport) {
	report.Generation = it.Sensor.Generation
	h.tree.Dispatch(driver.Event{Kind: driver.EventSensor, Count: it.Count, Data: &report})
}

func TestSensorPermissionIsExplicitAndScoped(t *testing.T) {
	h, cx := sensorHost(t)
	var grants []SensorGrant
	s, err := cx.ReadSensorPermission(SensorLocation, func(g SensorGrant) { grants = append(grants, g) })
	if err != nil {
		t.Fatal(err)
	}
	it := command(h.commit(), driver.ItemSensor)
	if it.Sensor.Command != driver.SensorReadPermission {
		t.Fatal("read prompted")
	}
	sensorAnswer(h, it, driver.SensorReport{Grant: &SensorGrant{Authorization: SensorGranted, Precise: false}})
	if len(grants) != 1 || grants[0].Precise || !s.Stopped() {
		t.Fatalf("grants %v, stopped %v", grants, s.Stopped())
	}
	h.commit()
	pending, _ := cx.RequestSensorPermission(SensorMotion, func(g SensorGrant) { grants = append(grants, g) })
	it = command(h.commit(), driver.ItemSensor)
	if it.Sensor.Command != driver.SensorRequestPermission {
		t.Fatal("request did not ask")
	}
	pending.Stop()
	sensorAnswer(h, it, driver.SensorReport{Grant: &SensorGrant{Authorization: SensorGranted}})
	if len(grants) != 1 {
		t.Fatal("canceled permission reached user code")
	}
}

func TestSensorPauseRejectsOldGenerationAndMarksGap(t *testing.T) {
	h, cx := sensorHost(t)
	var fixes []LocationFix
	s, err := cx.WatchLocation(LocationOptions{}, func(f LocationFix) { fixes = append(fixes, f) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := command(h.commit(), driver.ItemSensor)
	sensorAnswer(h, first, driver.SensorReport{Location: &LocationFix{Latitude: 1}})
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleInactive})
	if command(h.commit(), driver.ItemSensor) != nil {
		t.Fatal("inactive paused a visible stream")
	}
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleBackground})
	if it := command(h.commit(), driver.ItemSensor); it == nil || it.Sensor.Command != driver.SensorStop {
		t.Fatal("background did not stop provider")
	}
	sensorAnswer(h, first, driver.SensorReport{Location: &LocationFix{Latitude: 2}})
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleActive})
	next := command(h.commit(), driver.ItemSensor)
	if next.Sensor.Generation == first.Sensor.Generation {
		t.Fatal("restart reused generation")
	}
	sensorAnswer(h, first, driver.SensorReport{Location: &LocationFix{Latitude: 3}})
	sensorAnswer(h, next, driver.SensorReport{Location: &LocationFix{Latitude: 4}})
	sensorAnswer(h, next, driver.SensorReport{Location: &LocationFix{Latitude: 5}})
	if len(fixes) != 3 || fixes[1].Latitude != 4 || !fixes[1].Gap || fixes[2].Gap {
		t.Fatalf("fixes %+v", fixes)
	}
	s.Stop()
	h.commit()
	sensorAnswer(h, next, driver.SensorReport{Location: &LocationFix{Latitude: 6}})
	if len(fixes) != 3 {
		t.Fatal("stopped stream delivered")
	}
}

func TestSensorOneShotTimeoutWaitsThroughPause(t *testing.T) {
	h, cx := sensorHost(t)
	var got error
	s, _ := cx.Locate(LocationOptions{Timeout: time.Second}, func(_ LocationFix, err error) { got = err })
	h.commit()
	h.clock.Advance(400 * time.Millisecond)
	h.drain()
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleBackground})
	h.commit()
	h.clock.Advance(time.Hour)
	h.drain()
	if s.Stopped() || got != nil {
		t.Fatal("timeout ran in background")
	}
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleActive})
	h.commit()
	h.clock.Advance(599 * time.Millisecond)
	h.drain()
	if s.Stopped() {
		t.Fatal("timeout did not preserve remaining time")
	}
	h.clock.Advance(time.Millisecond)
	h.drain()
	if !errors.Is(got, SensorTimedOut) || !s.Stopped() {
		t.Fatalf("timeout %v, stopped %v", got, s.Stopped())
	}
}

func TestSensorOneShotTimeoutKeepsRunningWhileInactive(t *testing.T) {
	h, cx := sensorHost(t)
	var got error
	s, _ := cx.Locate(LocationOptions{Timeout: time.Second}, func(_ LocationFix, err error) { got = err })
	h.commit()
	h.clock.Advance(400 * time.Millisecond)
	h.drain()
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleInactive})
	h.commit()
	h.clock.Advance(600 * time.Millisecond)
	h.drain()
	if !s.Stopped() || !errors.Is(got, SensorTimedOut) {
		t.Fatalf("inactive timeout: %v, stopped %v", got, s.Stopped())
	}
}

func TestSensorOneShotAndOwnerDisposal(t *testing.T) {
	h, cx := sensorHost(t)
	var calls int
	s, _ := cx.Locate(LocationOptions{}, func(_ LocationFix, err error) {
		if err != nil {
			t.Fatal(err)
		}
		calls++
	})
	it := command(h.commit(), driver.ItemSensor)
	sensorAnswer(h, it, driver.SensorReport{Location: &LocationFix{Latitude: 1}})
	sensorAnswer(h, it, driver.SensorReport{Location: &LocationFix{Latitude: 2}})
	if calls != 1 || !s.Stopped() {
		t.Fatal("one-shot delivered more than once")
	}
	h.commit()
	child := cx.Child()
	stream, _ := child.WatchMotion(MotionOptions{}, func(MotionSample) { calls++ }, func(error) { calls++ })
	it = command(h.commit(), driver.ItemSensor)
	child.Dispose()
	sensorAnswer(h, it, driver.SensorReport{Motion: &MotionSample{Sensor: SensorAccelerometer}})
	sensorAnswer(h, it, driver.SensorReport{Error: "not-allowed", End: true})
	if !stream.Stopped() || calls != 1 {
		t.Fatal("disposed owner received callback")
	}
}

func TestSensorBackgroundOptInAndRateClamp(t *testing.T) {
	h, cx := sensorHost(t)
	stream, _ := cx.WatchLocation(LocationOptions{Background: true}, nil, nil)
	h.commit()
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleBackground})
	if command(h.commit(), driver.ItemSensor) != nil || stream.paused {
		t.Fatal("explicit background stream paused")
	}
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleActive})
	m, err := cx.WatchMotion(MotionOptions{Sensors: SensorAccelerometer | SensorDeviceMotion, Interval: time.Nanosecond}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	it := command(h.commit(), driver.ItemSensor)
	if it.Sensor.Motion.Interval != 10*time.Millisecond || it.Sensor.Motion.Sensors != SensorAccelerometer|SensorDeviceMotion {
		t.Fatalf("motion %+v", it.Sensor.Motion)
	}
	if m.Stopped() {
		t.Fatal("stream ended before provider answer")
	}
	for _, options := range []LocationOptions{{Accuracy: math.NaN()}, {Distance: -1}, {MaxAge: -1}} {
		if _, err := cx.Locate(options, nil); err == nil {
			t.Fatal("invalid location options accepted")
		}
	}
	if _, err := cx.WatchMotion(MotionOptions{Sensors: 128}, nil, nil); err == nil {
		t.Fatal("unknown sensor accepted")
	}
}

type sensorAccess map[string]bool

func (a sensorAccess) Allows(r AccessRequest) bool { return a[r.Permission] }
func (sensorAccess) WebHosts() []string            { return nil }

func TestSensorCapabilitiesAreIndependent(t *testing.T) {
	h, cx := sensorHost(t)
	cx.ProvideAccess(sensorAccess{AccessLocation: true})
	if _, err := cx.WatchMotion(MotionOptions{}, nil, nil); err == nil {
		t.Fatal("location granted motion")
	}
	if _, err := cx.WatchLocation(LocationOptions{Background: true}, nil, nil); err == nil {
		t.Fatal("foreground granted background")
	}
	if command(h.commit(), driver.ItemSensor) != nil {
		t.Fatal("denied capability reached provider")
	}
	if _, err := cx.WatchLocation(LocationOptions{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if command(h.commit(), driver.ItemSensor) == nil {
		t.Fatal("allowed location did not start")
	}
}
