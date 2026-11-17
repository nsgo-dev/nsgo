package navheader_test

import (
	"image/color"
	"testing"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestCustomHeaderNativePixels(t *testing.T) {
	switch nsgotest.Target() {
	case "android", "ios", "ios_simulator":
	default:
		t.Skip("requires a native mobile navigation bar")
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		nav := base.NewNav(cx, "home")
		return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, _ string, _ func() int) base.NavPage {
			return ui.NavPage(cx, base.NavPage{Title: "Page", TestID: "page",
				Header:   core.View{TestID: "custom", Style: core.Style{Width: core.Pt(120), Height: core.Pt(24), Background: core.RGB(0xff00ff)}},
				Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1}}},
			}, ui.NavHeader{})
		})
	})
	if app.Find("id:page").Screen.TitleView == 0 || len(app.QueryMounted("id:custom")) != 1 {
		t.Fatal("the custom header is not the system title view")
	}
	img := app.Capture()
	pixels := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			// UIKit captures may encode the device's color space rather than sRGB.
			if c.R > 180 && c.B > 180 && c.G < 80 && c.A > 240 {
				pixels++
			}
		}
	}
	if pixels < 500 {
		app.Screenshot("custom-header-native.png")
		t.Fatalf("only %d custom-header pixels in the native capture", pixels)
	}
}
