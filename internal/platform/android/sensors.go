package android

import (
	"encoding/json"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func writeSensorRequest(w *writer, id uint32, r driver.SensorRequest) {
	w.u32(id)
	w.byte(byte(r.Command))
	w.byte(byte(r.Kind))
	w.u32(r.Generation)
	w.f64(r.Location.Accuracy)
	w.f64(r.Location.Distance)
	w.u64(uint64(r.Location.Interval))
	w.u64(uint64(r.Location.MaxAge))
	w.flags(r.Location.Background)
	w.str(r.Location.NotificationTitle)
	w.str(r.Location.NotificationBody)
	w.byte(byte(r.Motion.Sensors))
	w.u64(uint64(r.Motion.Interval))
}

// sensorReports coalesces pipe input before Engine.Dispatch. The Java
// writer separately coalesces while the pipe itself is backed up.
type sensorReports struct{ streams map[uint32]*sensorPackets }
type sensorPackets struct {
	generation uint32
	packets    [5]driver.SensorPacket
}

func (s *sensorReports) receive(payload []byte, dispatch func(driver.Event)) {
	var r struct {
		ID uint32 `json:"id"`
		driver.SensorReport
	}
	if json.Unmarshal(payload, &r) != nil {
		return
	}
	if r.End || r.Grant != nil || r.Error != "" {
		delete(s.streams, r.ID)
		dispatch(driver.Event{Kind: driver.EventSensor, Count: r.ID, Data: &r.SensorReport})
		return
	}
	key := 0
	if r.Motion != nil {
		switch r.Motion.Sensor {
		case driver.SensorAccelerometer:
			key = 1
		case driver.SensorGyroscope:
			key = 2
		case driver.SensorMagnetometer:
			key = 3
		case driver.SensorDeviceMotion:
			key = 4
		default:
			return
		}
	} else if r.Location == nil {
		return
	}
	if s.streams == nil {
		s.streams = map[uint32]*sensorPackets{}
	}
	stream := s.streams[r.ID]
	if stream == nil || stream.generation != r.Generation {
		stream = &sensorPackets{generation: r.Generation}
		s.streams[r.ID] = stream
	}
	p := &stream.packets[key]
	if p.Offer(r.SensorReport) {
		dispatch(driver.Event{Kind: driver.EventSensor, Count: r.ID, Data: p})
	}
}
