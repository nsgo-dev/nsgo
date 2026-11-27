package processstart

import "testing"

func TestProcessStartTicks(t *testing.T) {
	const fields = " S 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 "
	for _, tc := range []struct {
		name string
		stat string
		want int64
		ok   bool
	}{
		{"normal", "123 (go)" + fields + "3456789 23\n", 3456789, true},
		{"embedded delimiter", "123 (nested ) S (parens)\nmore)" + fields + "987654321 23", 987654321, true},
		{"empty name and zero ticks", "123 ()" + fields + "0 23", 0, true},
		{"truncated", "123 (go)" + fields, 0, false},
		{"negative", "123 (go)" + fields + "-1 23", 0, false},
		{"max signed", "123 (go)" + fields + "9223372036854775807 23", 1<<63 - 1, true},
		{"signed overflow", "123 (go)" + fields + "9223372036854775808 23", 0, false},
		{"missing name", "123 go)" + fields + "10", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := procStartTicks([]byte(tc.stat))
			if ok != tc.ok || ok && got != tc.want {
				t.Errorf("procStartTicks = (%d, %t), want (%d, %t)", got, ok, tc.want, tc.ok)
			}
		})
	}
}
