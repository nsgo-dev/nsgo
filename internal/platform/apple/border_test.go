//go:build darwin

package apple

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestMoveBordersAboveBackdropLayer(t *testing.T) {
	layer := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	backdrop := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	defer layer.Send(selRelease)
	defer backdrop.Send(selRelease)
	layer.Send(sel("addSublayer:"), backdrop)
	setBorders(layer, &driver.ViewStyle{Borders: driver.Edges{Top: 1}, BorderColors: driver.EdgeColors{Top: driver.RGB(0xff0000)}}, nil)
	r := rings[layer]
	if r == nil || r.sides[0] == 0 {
		t.Fatal("per-side border did not create a ring")
	}
	MoveBorders(layer, backdrop)
	if r.sides[0].Send(sel("superlayer")) != backdrop {
		t.Fatal("border remained under the backdrop")
	}
	MoveBorders(layer, 0)
	if r.sides[0].Send(sel("superlayer")) != layer {
		t.Fatal("border did not return to the view layer")
	}
	setBorders(layer, &driver.ViewStyle{}, nil)
}
