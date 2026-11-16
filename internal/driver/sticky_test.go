package driver

import "testing"

func TestStickyShiftAtBothEnds(t *testing.T) {
	for _, c := range []struct {
		name            string
		cell            StickyCell
		offsets, shifts []Points
	}{
		{"start", StickyCell{Start: 100, End: 300}, []Points{0, 100, 150, 400}, []Points{0, 0, 50, 200}},
		{"end", StickyCell{Start: 840, End: -160}, []Points{-500, 0, 500, 840, 1000}, []Points{-1000, -840, -340, 0, 0}},
		{"short", StickyCell{Start: -70, End: -170}, []Points{0, 100}, []Points{0, 0}},
		{"no travel", StickyCell{Start: 100, End: 100}, []Points{0, 100, 200}, []Points{0, 0, 0}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for i, offset := range c.offsets {
				if got := c.cell.StickyShift(offset); got != c.shifts[i] {
					t.Errorf("offset %v: shift %v, want %v", offset, got, c.shifts[i])
				}
			}
		})
	}
}
