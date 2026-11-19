package canvastest

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// RedPNG is a tiny red PNG.
func RedPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// cacheSources counts the images CacheSources made, so a later call's differ from an earlier one's.
var cacheSources atomic.Uint64

// CacheSources returns n tiny red images for cache-pressure tests, distinct from each other and from any it returned before: each PNG carries a number of its own in a text chunk, so they draw alike but their bytes, and so their keys, differ.
func CacheSources(n int) []driver.ImageSource {
	data := RedPNG()
	// The chunks after the signature and IHDR, where a tEXt chunk may go.
	const at = 8 + 4 + 4 + 13 + 4
	sources := make([]driver.ImageSource, n)
	for i := range sources {
		text := append([]byte("nsgo\x00"), strconv.FormatUint(cacheSources.Add(1), 10)...)
		chunk := binary.BigEndian.AppendUint32(nil, uint32(len(text)))
		chunk = append(chunk, "tEXt"...)
		chunk = append(chunk, text...)
		chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
		sources[i].Data = slices.Concat(data[:at], chunk, data[at:])
	}
	return sources
}
