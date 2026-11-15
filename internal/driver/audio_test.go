package driver

import "testing"

func TestAudioType(t *testing.T) {
	for _, tc := range []struct {
		data string
		want string
	}{
		{"RIFF\x24\x00\x00\x00WAVEfmt ", "audio/wav"},
		{"FORM\x00\x00\x00\x00AIFC", "audio/aiff"},
		{"caff\x00\x01", "audio/x-caf"},
		{"fLaC\x00", "audio/flac"},
		{"OggS\x00\x02", "audio/ogg"},
		{"\x00\x00\x00\x20ftypM4A ", "audio/mp4"},
		{"ID3\x04\x00", "audio/mpeg"},
		{"\xFF\xFB\x90\x64", "audio/mpeg"},
		{"\xFF\xF1\x50\x80", "audio/aac"},
		{"\xFF\x00", "application/octet-stream"},
		{"RIFF", "application/octet-stream"},
		{"", "application/octet-stream"},
	} {
		if got := AudioType([]byte(tc.data)); got != tc.want {
			t.Errorf("AudioType(%q) = %s, want %s", tc.data, got, tc.want)
		}
	}
}
