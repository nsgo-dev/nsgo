package driver

import (
	"slices"
	"testing"
)

func TestLiveUpdates(t *testing.T) {
	SendLiveUpdate([]byte("a"))
	ReportLiveUpdate([]byte("x"))
	var sent, heard []string
	ConnectLiveUpdates(func(m []byte) { sent = append(sent, string(m)) })
	SendLiveUpdate([]byte("b"))
	HearLiveUpdates(func(r []byte) { heard = append(heard, string(r)) })
	ReportLiveUpdate([]byte("y"))
	if !slices.Equal(sent, []string{"a", "b"}) || !slices.Equal(heard, []string{"x", "y"}) {
		t.Errorf("sent %v and heard %v, want what waited first", sent, heard)
	}
}

func TestRecordings(t *testing.T) {
	t.Cleanup(func() { ConnectRecordings(nil) })
	SendRecording([]byte("a"))
	ReportRecording([]byte("x"))
	var sent, heard []string
	ConnectRecordings(func(m []byte) { sent = append(sent, string(m)) })
	if !RecordingsConnected() {
		t.Error("a connected recorder is not")
	}
	SendRecording([]byte("b"))
	HearRecordings(func(r []byte) { heard = append(heard, string(r)) })
	ReportRecording([]byte("y"))
	ConnectRecordings(nil)
	if RecordingsConnected() {
		t.Error("a disconnected recorder is connected")
	}
	SendRecording([]byte("c"))
	ConnectRecordings(func(m []byte) { sent = append(sent, string(m)) })
	if !slices.Equal(sent, []string{"a", "b", "c"}) || !slices.Equal(heard, []string{"x", "y"}) {
		t.Errorf("sent %v and heard %v, want what waited first", sent, heard)
	}
}
