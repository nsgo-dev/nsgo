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
  <View testID="permission" style={{height:20}} onPress={() => Sensors.permission("location").then(s => state.set("permission " + s))} />
  <View testID="ask" style={{height:20}} onPress={() => Sensors.requestPermission("motion").then(s => state.set("asked " + s))} />
  <View testID="locate" style={{height:20}} onPress={() => Sensors.locate().then(f => state.set("fix " + f.latitude + " " + f.altitude + " " + f.cached + " " + f.approximate))} />
  <View testID="motion" style={{height:20}} onPress={() => stop = Sensors.watchMotion({sensors:1, interval:1}, m => state.set("motion " + m.vector.z + " " + m.interval + " " + m.dropped))} />
  <View testID="stop" style={{height:20}} onPress={() => stop()} />
  <Text testID="state">{state}</Text>
 </View>;
}
mount(App);`)
	app.Permissions[core.PermissionLocation] = core.PermissionGranted
	app.PermissionAnswers[core.PermissionMotion] = core.PermissionGranted
	app.Press("id:permission")
	app.WaitFor("text:permission granted", time.Second)
	app.Press("id:ask")
	app.WaitFor("text:asked granted", time.Second)
	app.Press("id:locate")
	app.FeedLocation(core.LocationFix{Latitude: 42, Timestamp: app.Clock.Now(), Approximate: true})
	app.Settle()
	app.WaitFor("text:fix 42 null false true", time.Second)
	app.Press("id:motion")
	app.FeedMotion(core.MotionSample{Sensor: core.SensorAccelerometer, Vector: core.SensorVector{Z: 9.81}})
	app.Settle()
	app.WaitFor("text:motion 9.81 10 0", time.Second)
	app.Press("id:stop")
	if app.ActiveSensorStreams() != 0 {
		t.Fatal("TSX stop retained a provider")
	}
}
