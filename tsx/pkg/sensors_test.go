package pkg

import "testing"

func TestSensorCapabilitiesParseAndStayIndependent(t *testing.T) {
	c, ignored, err := ParseCapabilities([]byte(`{"location":true,"backgroundLocation":false,"motion":true}`), true)
	if err != nil || len(ignored) != 0 {
		t.Fatalf("capabilities: %v %v", ignored, err)
	}
	if !c.Allows("location", "", "") || !c.Allows("motion", "", "") || c.Allows("backgroundLocation", "", "") {
		t.Fatal("sensor capabilities were conflated")
	}
	if _, _, err := ParseCapabilities([]byte(`{"location":"yes"}`), true); err == nil {
		t.Fatal("invalid sensor capability accepted")
	}
	c, _, err = ParseCapabilities([]byte(`{"location":true,"backgroundLocation":true}`), true)
	if err != nil || !c.Allows("backgroundLocation", "", "") || c.Allows("motion", "", "") {
		t.Fatal("background capability granted motion")
	}
}
