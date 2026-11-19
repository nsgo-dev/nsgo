package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// keepsClear fails unless every text under selector, and every view selectors name, lies between the left and right safe insets of a window width wide.
func keepsClear(t *testing.T, app *nsgotest.App, what string, width core.Points, safe core.Edges, selector string, more ...string) {
	t.Helper()
	inside := func(v *nsgotest.View) bool {
		b := v.Bounds()
		return b.Origin.X >= safe.Left-0.5 && b.Origin.X+b.Size.Width <= width-safe.Right+0.5
	}
	texts := 0
	var walk func(v *nsgotest.View)
	walk = func(v *nsgotest.View) {
		if v.Text != "" && v.Frame.Size.Width > 0 {
			texts++
			if !inside(v) {
				t.Errorf("%s: %q is at %v, outside %v..%v", what, v.Text, v.Bounds(), safe.Left, width-safe.Right)
			}
		}
		for _, c := range v.Children {
			walk(c)
		}
	}
	walk(app.Find(selector))
	if texts == 0 {
		t.Fatalf("%s: no text under %s", what, selector)
	}
	for _, s := range more {
		if v := app.Find(s); !inside(v) {
			t.Errorf("%s: %s is at %v, outside %v..%v", what, s, v.Bounds(), safe.Left, width-safe.Right)
		}
	}
}

// TestPhoneKeepsClearOfTheSides keeps the narrow layout's index and pages clear of a right safe inset, the vertical bar beside a foldable's outer display, and of a left one, an iPhone's camera in landscape, without padding a page that keeps clear of the safe area itself twice.
func TestPhoneKeepsClearOfTheSides(t *testing.T) {
	// A target's window has the safe area of the screen it is on, not the one nsgotest.Environment sets.
	headless(t)
	for _, tc := range []struct {
		name          string
		width, height core.Points
		safe          core.Edges
	}{
		{"outer display", 466, 678, core.Edges{Top: 20, Right: 84}},
		{"landscape", 874, 402, core.Edges{Right: 62, Bottom: 21, Left: 62}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := nsgotest.Start(t, Root, nsgotest.Size(tc.width, tc.height),
				nsgotest.Environment(core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US", SafeArea: tc.safe}))
			keepsClear(t, app, "index", tc.width, tc.safe, "id:home", "id:nav-Stocks")
			app.Press("id:nav-Stocks")
			keepsClear(t, app, "stocks", tc.width, tc.safe, "id:screen-Stocks", "id:stock-ORBX", "id:watchlist")
			app.Back()
			openPhone(app, "NavStack")
			// The headerless page pads by the safe area on every side itself.
			app.Press("role:button No header")
			if x, want := app.Find("text:/The screen owns the whole surface/").Bounds().Origin.X, tc.safe.Left+16+4; x != want {
				t.Errorf("the headerless page's text starts at %v, want %v: the left inset once and the page's padding", x, want)
			}
		})
	}
}

// TestDesktopLayoutKeepsClearOfTheSides keeps the wide layout's sidebar clear of a left safe inset and its content of a right one, and of the left one once the sidebar closes.
func TestDesktopLayoutKeepsClearOfTheSides(t *testing.T) {
	// A target's window has the safe area of the screen it is on, not the one nsgotest.Environment sets.
	headless(t)
	const width, height = 956, 440
	safe := core.Edges{Right: 84, Bottom: 21, Left: 62}
	app := nsgotest.Start(t, Root, nsgotest.Size(width, height),
		nsgotest.Environment(core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US", SafeArea: safe}))
	open(app, "Stocks")
	sidebar := safe
	sidebar.Right = 0
	keepsClear(t, app, "sidebar", width, sidebar, "id:sidebar", "id:nav-Stocks", "role:button Settings")
	content := safe
	content.Left = 0
	keepsClear(t, app, "content", width, content, "id:demo-Stocks", "id:stock-ORBX", "id:stock-price")
	if x := app.Find("id:demo-Stocks").Bounds().Origin.X; x < sidebarWidth || x > sidebarWidth+2 {
		t.Errorf("with the sidebar open the content starts at %v, want beside the sidebar", x)
	}
	app.Press("role:button Toggle sidebar")
	keepsClear(t, app, "content without the sidebar", width, safe, "id:demo-Stocks", "id:stock-ORBX", "id:stock-price")
}
