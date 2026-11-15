//go:build darwin

package apple

import (
	"math"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestSensorAppleAccelerationAndNorthFrame(t *testing.T) {
	v := motionVector(cmVector{Z: -1}, -9.80665)
	if v.X != 0 || v.Y != 0 || math.Abs(v.Z-9.80665) > 1e-9 {
		t.Fatalf("face-up acceleration %+v", v)
	}
	q := northToENU(driver.SensorQuaternion{W: 1})
	if math.Abs(q.Z-math.Sqrt2/2) > 1e-9 || math.Abs(q.W-math.Sqrt2/2) > 1e-9 {
		t.Fatalf("north frame %+v", q)
	}
	if sensorNumber(-1, false).Valid || !sensorNumber(0, true).Valid || sensorNumber(math.NaN(), true).Valid {
		t.Fatal("missing measurements confused with zero")
	}
}

// A CLLocation constructed from a fixture, never a manager querying the
// machine's location, exercises Objective-C struct returns and timestamps.
func TestSensorAppleLocationFixtureRejectsOldFixByDefault(t *testing.T) {
	if !loadLocation() {
		t.Skip("Core Location unavailable")
	}
	pool := poolPush()
	defer poolPop(pool)
	date := objc.ID(Class("NSDate")).Send(sel("dateWithTimeIntervalSince1970:"), float64(1000))
	fixture := objc.ID(Class("CLLocation")).Send(sel("alloc")).Send(sel("initWithCoordinate:altitude:horizontalAccuracy:verticalAccuracy:timestamp:"), locationCoordinate{Latitude: 51.5, Longitude: -0.12}, float64(35), float64(5), float64(-1), date)
	if fixture == 0 {
		t.Fatal("could not create fixture")
	}
	defer fixture.Send(selRelease)
	r := &appleSensorRun{r: driver.SensorRequest{Command: driver.SensorLocate}, started: time.Unix(1001, 0)}
	if _, ok := r.location(fixture); ok {
		t.Fatal("old fix accepted without MaxAge")
	}
	r.r.Location.MaxAge = 100 * 365 * 24 * time.Hour
	f, ok := r.location(fixture)
	if !ok || f.Latitude != 51.5 || f.Longitude != -0.12 || !f.Cached || f.Timestamp.Unix() != 1000 || f.HasMonotonic || f.VerticalAccuracy.Valid || f.MSLAltitude.Valid || f.HorizontalAccuracy.Value != 5 {
		t.Fatalf("fix %+v, accepted %v", f, ok)
	}
}
