//go:build darwin

package apple

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/shader"
)

// TestShadersReadImages draws a shader that reads an image into a texture
// with Metal and reads the pixels back: the image's left half red, its
// right half blue, stretched over the target and tinted by a uniform.
func TestShadersReadImages(t *testing.T) {
	if !loadMetal() {
		t.Skip("no Metal device")
	}
	ir, err := shader.CompileFragment([]byte(`package main

var Alpha float

func Fragment(dst vec4, pos vec2) vec4 {
	return image0At(pos*image0Size()/rectSize()) * Alpha
}
`))
	if err != nil {
		t.Fatal(err)
	}
	// A 4 by 2 image: red on the left, blue on the right.
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		for y := range 2 {
			c := color.RGBA{255, 0, 0, 255}
			if x >= 2 {
				c = color.RGBA{0, 0, 255, 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	var encoded bytes.Buffer
	png.Encode(&encoded, src)
	d, err := decode(context.Background(), encoded.Bytes(), request{ratio: 1, scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	tex := textureOf(d.image, 4, 2)
	if tex == 0 {
		t.Fatal("no texture")
	}
	defer tex.Send(selRelease)
	const w, h = 8, 4
	lanes := []float64{w, h, 1, 4, 2, 0.5}
	pix := drawMetal(t, ir, lanes, []objc.ID{tex}, w, h)
	// BGRA, premultiplied, at half alpha.
	left, right := pix[(1*w+1)*4:], pix[(1*w+6)*4:]
	if left[2] < 120 || left[0] > 10 || left[3] < 120 || left[3] > 135 {
		t.Errorf("left pixel %v, want half red", left[:4])
	}
	if right[0] < 120 || right[2] > 10 {
		t.Errorf("right pixel %v, want half blue", right[:4])
	}
}
