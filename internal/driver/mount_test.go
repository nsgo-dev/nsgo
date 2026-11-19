package driver

import "testing"

// TestImageSourceKeyNamesDataByItsBytes keys inline data by its bytes, not its address: a collected buffer's address can come back in a new buffer of the same length, whose other bytes must not name the cached image of the old ones (#738).
func TestImageSourceKeyNamesDataByItsBytes(t *testing.T) {
	data := []byte{1, 2, 3}
	before := ImageSource{Data: data}.Key()
	if again := (ImageSource{Data: append([]byte(nil), data...)}).Key(); again != before {
		t.Fatalf("the same bytes in another buffer have another key: %q, not %q", again, before)
	}
	data[0]++
	if after := (ImageSource{Data: data}).Key(); after == before {
		t.Fatalf("other bytes at the same address keep the key %q", after)
	}
}
