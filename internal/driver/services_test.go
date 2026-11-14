package driver

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestClipboardPNG(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	if c := ClipboardPNG(b.Bytes()); c == nil || c.Err != "" || c.Size != (Size{Width: 3, Height: 2}) {
		t.Fatalf("ClipboardPNG = %+v", c)
	}
	for _, data := range [][]byte{[]byte("GIF89a"), b.Bytes()[:20]} {
		if c := ClipboardPNG(data); c == nil || c.Err == "" {
			t.Errorf("ClipboardPNG(%q) = %+v, want an error", data, c)
		}
	}
}

func TestAnswerPermission(t *testing.T) {
	var got []Event
	dispatch := func(ev Event) { got = append(got, ev) }
	for _, c := range []PermissionCommand{PermissionCheck, PermissionAsk, PermissionOpenSettings, PermissionGuide, PermissionStopGuide} {
		AnswerPermission(uint32(c)+1, PermissionRequest{Command: c, Name: PermissionCamera}, PhonePermission, dispatch)
	}
	want := []Event{{Kind: EventPermission, Count: 1, Index: PermissionUnknown}, {Kind: EventPermission, Count: 2, Index: PermissionUnknown},
		{Kind: EventPermission, Count: 3}, {Kind: EventPermission, Count: 4, Index: PermissionUnknown}}
	if len(got) != len(want) {
		t.Fatalf("answers %+v", got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Count != want[i].Count || got[i].Index != want[i].Index || got[i].Finished {
			t.Errorf("answer %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if PhonePermission(PermissionAccessibility) != PermissionUnavailable {
		t.Error("a phone keeps Accessibility")
	}
}
