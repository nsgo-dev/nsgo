package tsx_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestSensorsTSXSubscriptionsAndPermissionPromises(t *testing.T) {
	if nsgotest.Target() != "" {
		t.Skip("synthetic sensor fixtures run headless only")
	}
	app := run(t, `import { Sensors, View, Text, mount, signal } from "nsgo";
function App() {
 const state = signal("idle"); let stop = () => {};
 return <View>
  <View testID="permission" style={{height:20}} onPress={() => Sensors.permission("location").then(g => state.set(g.authorization + " " + g.precise))} />
  <View testID="locate" style={{height:20}} onPress={() => Sensors.locate().then(f => state.set("fix " + f.latitude + " " + f.altitude + " " + f.cached))} />
  <View testID="motion" style={{height:20}} onPress={() => stop = Sensors.watchMotion({sensors:1, interval:1}, m => state.set("motion " + m.vector.z + " " + m.interval + " " + m.dropped))} />
  <View testID="stop" style={{height:20}} onPress={() => stop()} />
  <Text testID="state">{state}</Text>
 </View>;
}
mount(App);`)
	app.SensorPermissions = map[core.SensorKind]core.SensorGrant{
		core.SensorLocation: {Authorization: core.SensorGranted}, core.SensorMotion: {Authorization: core.SensorGranted},
	}
	app.Press("id:permission")
	app.WaitFor("text:granted false", time.Second)
	app.Press("id:locate")
	app.FeedLocation(core.LocationFix{Latitude: 42, Timestamp: app.Clock.Now()})
	app.Settle()
	app.WaitFor("text:fix 42 null false", time.Second)
	app.Press("id:motion")
	app.FeedMotion(core.MotionSample{Sensor: core.SensorAccelerometer, Vector: core.SensorVector{Z: 9.81}})
	app.Settle()
	app.WaitFor("text:motion 9.81 10 0", time.Second)
	app.Press("id:stop")
	if app.ActiveSensorStreams() != 0 {
		t.Fatal("TSX stop retained a provider")
	}
}
