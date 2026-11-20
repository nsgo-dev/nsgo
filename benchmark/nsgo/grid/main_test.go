package main

import "testing"

func TestGridDimension(t *testing.T) {
	for _, test := range []struct {
		name     string
		value    string
		fallback int
		want     int
	}{
		{name: "empty", value: "", fallback: 30, want: 30},
		{name: "positive", value: "40", fallback: 30, want: 40},
		{name: "zero", value: "0", fallback: 30, want: 30},
		{name: "negative", value: "-2", fallback: 30, want: 30},
		{name: "invalid", value: "many", fallback: 30, want: 30},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := gridDimension(test.value, test.fallback); got != test.want {
				t.Fatalf("gridDimension(%q, %d) = %d, want %d", test.value, test.fallback, got, test.want)
			}
		})
	}
}
