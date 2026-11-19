//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

// TestCanvasImageTellsNewBytesAtAnAddressApart loads an image again when other bytes take its inline data's place: a collected buffer's address can come back in a new buffer of the same length, which must not be handed the old image (#738).
func TestCanvasImageTellsNewBytesAtAnAddressApart(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("run with nsgo test -target web")
	}
	s := &surface{doc: doc}
	v := &view{id: 1}
	data := canvastest.RedPNG()
	im := s.loadImage(driver.ImageSource{Data: data}, v)
	if s.loadImage(driver.ImageSource{Data: append([]byte(nil), data...)}, v) != im {
		t.Fatal("the same bytes in another buffer loaded the image again")
	}
	data[len(data)-1]++
	if s.loadImage(driver.ImageSource{Data: data}, v) == im {
		t.Fatal("other bytes at the same address were handed the earlier image")
	}
}
