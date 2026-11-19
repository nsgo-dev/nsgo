package navheader_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

// TestTVNavTitle pushes pages on an Apple TV: the system header draws the title in the header's color, a large title's header is as tall as any other since tvOS draws no large titles, and the remote's Menu button pops the page.
func TestTVNavTitle(t *testing.T) {
	if nsgotest.Target() != "tvos_simulator" {
		t.Skip("needs an Apple TV: nsgo test -target tvos_simulator")
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		nav := base.NewNav(cx, "home")
		return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
			content := core.View{TestID: "content-" + route, Style: core.Style{FlexGrow: 1}}
			switch route {
			case "colored":
				return base.NavPage{Title: "Colored", TestID: "page-colored", HeaderStyle: core.Style{Color: core.RGB(0xff00ff), Background: core.RGB(0x000000)}, Children: []core.Node{content}}
			case "large":
				return ui.NavPage(cx, base.NavPage{Title: "Large", TestID: "page-large", HeaderLargeTitle: true, Children: []core.Node{content}}, ui.NavHeader{})
			}
			return ui.NavPage(cx, base.NavPage{Title: "Home", TestID: "page-home", Children: []core.Node{
				ui.Button{Text: "Colored", TestID: "colored", OnPress: func() { nav.Push("colored") }},
				ui.Button{Text: "Large", TestID: "large", OnPress: func() { nav.Push("large") }},
			}}, ui.NavHeader{})
		})
	})
	regular := app.Find("id:page-home").Screen.HeaderHeight

	app.Press("id:colored")
	app.WaitFor("id:content-colored", 5*time.Second)
	app.FastForward(time.Second)
	img := app.Capture()
	page := app.Find("id:page-colored").Frame.Size
	if page.Height <= 0 || regular <= 0 {
		t.Fatalf("the page is %v with a header %v tall", page, regular)
	}
	header := min(int(float64(regular)*float64(img.Bounds().Dy())/float64(page.Height)), img.Bounds().Dy())
	pixels := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Min.Y+header; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			// UIKit captures may encode the device's color space rather than sRGB.
			if c.R > 180 && c.B > 180 && c.G < 80 && c.A > 240 {
				pixels++
			}
		}
	}
	if pixels < 500 {
		app.Screenshot("tv-colored-title")
		t.Fatalf("only %d pixels of the title's color in the header", pixels)
	}
	app.Key("escape")
	app.WaitForGone("id:content-colored", 5*time.Second)

	app.Press("id:large")
	app.WaitFor("id:content-large", 5*time.Second)
	if got := app.Find("id:page-large").Screen.HeaderHeight; got != regular {
		t.Fatalf("a large title's header is %v tall, want the bar's %v", got, regular)
	}
	app.Key("escape")
	app.WaitForGone("id:content-large", 5*time.Second)
	app.WaitFor("id:colored", 5*time.Second)
}
