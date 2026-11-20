package main

import (
	"slices"
	"testing"
)

func TestGridEnvironment(t *testing.T) {
	if got := gridEnvironment("quotes", 20, 15); len(got) != 0 {
		t.Fatalf("gridEnvironment for quotes = %v, want no overrides", got)
	}
	if got, want := gridEnvironment("grid", 20, 15), []string{"NSGO_GRID_COLS=20", "NSGO_GRID_ROWS=15"}; !slices.Equal(got, want) {
		t.Fatalf("gridEnvironment = %v, want %v", got, want)
	}
	if got, want := gridEnvironment("grid", 0, 15), []string{"NSGO_GRID_ROWS=15"}; !slices.Equal(got, want) {
		t.Fatalf("gridEnvironment with default columns = %v, want %v", got, want)
	}
}
