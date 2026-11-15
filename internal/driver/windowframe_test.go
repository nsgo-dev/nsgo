package driver

import "testing"

func TestWindowFrameOnScreen(t *testing.T) {
	main := Bounds{Origin: Point{X: 0, Y: 25}, Size: Size{Width: 1440, Height: 800}}
	left := Bounds{Origin: Point{X: -1920, Y: 0}, Size: Size{Width: 1920, Height: 1080}}
	screens := []Bounds{main, left}
	centered := WindowFrame{X: 320, Y: 225, Move: true, Width: 800, Height: 400}
	cases := []struct {
		name    string
		frame   WindowFrame
		screens []Bounds
		want    WindowFrame
	}{
		{"on the main screen", WindowFrame{X: 50, Y: 60, Move: true, Width: 800, Height: 400}, screens, WindowFrame{X: 50, Y: 60, Move: true, Width: 800, Height: 400}},
		{"on another screen", WindowFrame{X: -900, Y: 100, Move: true, Width: 800, Height: 400}, screens, WindowFrame{X: -900, Y: 100, Move: true, Width: 800, Height: 400}},
		{"past a screen's bottom", WindowFrame{X: 50, Y: 700, Move: true, Width: 800, Height: 400}, screens, WindowFrame{X: 50, Y: 700, Move: true, Width: 800, Height: 400}},
		{"on a screen that went away", WindowFrame{X: 3000, Y: 100, Move: true, Width: 800, Height: 400}, screens, centered},
		{"its top edge above the screens", WindowFrame{X: 50, Y: -300, Move: true, Width: 800, Height: 400}, []Bounds{main}, centered},
		{"barely on a screen", WindowFrame{X: 1400, Y: 100, Move: true, Width: 800, Height: 400}, []Bounds{main}, centered},
		{"larger than the screen left", WindowFrame{X: 3000, Y: 0, Move: true, Width: 2000, Height: 1000}, []Bounds{main}, WindowFrame{X: 0, Y: 25, Move: true, Width: 1440, Height: 800}},
		{"without screens", WindowFrame{X: 3000, Y: 0, Move: true, Width: 800, Height: 400}, nil, WindowFrame{X: 3000, Y: 0, Move: true, Width: 800, Height: 400}},
	}
	for _, c := range cases {
		if got := c.frame.OnScreen(c.screens); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestWindowFrameWithin(t *testing.T) {
	area := Bounds{Origin: Point{X: 0, Y: 24}, Size: Size{Width: 400, Height: 800}}
	for _, c := range []struct{ frame, want WindowFrame }{
		{WindowFrame{X: 10, Y: 40, Move: true, Width: 100, Height: 50}, WindowFrame{X: 10, Y: 40, Move: true, Width: 100, Height: 50}},
		{WindowFrame{X: -30, Y: 0, Move: true, Width: 100, Height: 50}, WindowFrame{X: 0, Y: 24, Move: true, Width: 100, Height: 50}},
		{WindowFrame{X: 350, Y: 790, Move: true, Width: 100, Height: 50}, WindowFrame{X: 300, Y: 774, Move: true, Width: 100, Height: 50}},
		{WindowFrame{X: 5, Y: 5, Move: true, Width: 900, Height: 2000}, WindowFrame{X: 0, Y: 24, Move: true, Width: 400, Height: 800}},
		{WindowFrame{Width: 100, Height: 50}, WindowFrame{X: 150, Y: 399, Width: 100, Height: 50}},
	} {
		if got := c.frame.Within(area); got != c.want {
			t.Errorf("%+v within %+v = %+v, want %+v", c.frame, area, got, c.want)
		}
	}
}
