package videoalpha

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/platform/videoalpha/clips"
)

// write writes b to a file named name in a test's directory.
func write(t *testing.T, name string, b []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileFindsTheDeclaredAlpha(t *testing.T) {
	for _, c := range []struct {
		name string
		b    []byte
		want Codec
	}{
		{"vp8a.webm", clips.VP8Alpha, VP8},
		{"vp9a.webm", clips.VP9Alpha, VP9},
		{"vp9.webm", clips.VP9, None},
		{"hevca.mov", clips.HEVCAlpha, HEVC},
		{"hevc.mov", clips.HEVC, None},
	} {
		if got := File(write(t, c.name, c.b)); got != c.want {
			t.Errorf("File(%s) = %v, want %v", c.name, got, c.want)
		}
	}
	if got := File(filepath.Join(t.TempDir(), "missing.webm")); got != None {
		t.Errorf("a missing file is %v", got)
	}
}

// A file cut anywhere, or with garbage in its headers, declares nothing and does not panic.
func TestReadSurvivesBrokenFiles(t *testing.T) {
	for _, b := range [][]byte{clips.VP9Alpha, clips.HEVCAlpha} {
		for n := range len(b) {
			Read(bytes.NewReader(b[:n]), int64(n))
			c := bytes.Clone(b)
			c[n] ^= 0xff
			Read(bytes.NewReader(c), int64(len(c)))
		}
	}
	if got := Read(bytes.NewReader([]byte("not a video at all")), 18); got != None {
		t.Errorf("text is %v", got)
	}
	// A box whose 64-bit size is near the largest int64, inside a movie box: its end overflows when added to its offset.
	huge := append(mp4Box("free", nil), 0, 0, 0, 1, 't', 'r', 'a', 'k', 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)
	if got := Read(bytes.NewReader(mp4Box("moov", huge)), int64(len(huge)+8)); got != None {
		t.Errorf("an overflowing box size is %v", got)
	}
}

// mp4Box is an ISO base media box of kind holding data.
func mp4Box(kind string, data []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(data)))
	return append(append(b, kind...), data...)
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r    io.ReaderAt
	read int
}

func (c *countingReader) ReadAt(b []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(b, off)
	c.read += n
	return n, err
}

// A movie box with megabytes of other boxes beside its tracks, as a long movie's sample tables are, is walked without being read whole.
func TestMovieReadsOnlyTheSampleDescriptions(t *testing.T) {
	r := bytes.NewReader(clips.HEVCAlpha)
	var moov []byte
	for off := int64(0); off < r.Size(); {
		kind, data, n, ok := box(r, off, r.Size())
		if !ok {
			t.Fatal("broken clip")
		}
		if kind == "moov" {
			moov = clips.HEVCAlpha[data : data+n]
		}
		off = data + n
	}
	if moov == nil {
		t.Fatal("no movie box")
	}
	file := mp4Box("moov", append(mp4Box("udta", make([]byte, 20<<20)), moov...))
	c := &countingReader{r: bytes.NewReader(file)}
	if got := Read(c, int64(len(file))); got != HEVC {
		t.Errorf("a 20 MiB movie box is %v, want HEVC", got)
	}
	if c.read > 64<<10 {
		t.Errorf("read %d bytes of a %d byte file", c.read, len(file))
	}
}

// The tracks of a segment whose size is unknown, as a live WebM stream writes it, are found too.
func TestMatroskaWithAnUnknownSegmentSize(t *testing.T) {
	b := bytes.Clone(clips.VP9Alpha)
	_, data, n, ok := element(bytes.NewReader(b), 0)
	if !ok {
		t.Fatal("no EBML header")
	}
	_, payload, _, ok := element(bytes.NewReader(b), data+n)
	if !ok {
		t.Fatal("no segment")
	}
	// The segment's size field, 8 bytes in libwebm's files, set to all ones.
	start := int(payload) - 8
	for i := start; i < int(payload); i++ {
		b[i] = 0xff
	}
	b[start] = 0x01
	if got := Read(bytes.NewReader(b), int64(len(b))); got != VP9 {
		t.Errorf("unknown-size segment: %v, want VP9", got)
	}
}

func TestLocal(t *testing.T) {
	root := filepath.FromSlash("/app/Resources")
	for _, c := range []struct {
		source, path string
		ok           bool
	}{
		{"asset:/clips/a.webm", filepath.Join(root, "clips", "a.webm"), true},
		{"asset:clips/a.webm", filepath.Join(root, "clips", "a.webm"), true},
		{"file:///tmp/a.webm", filepath.FromSlash("/tmp/a.webm"), true},
		{"https://example.com/a.webm", "", false},
		{"/tmp/a.webm", "/tmp/a.webm", true},
		{"", "", false},
	} {
		path, ok := Local(c.source, root)
		if path != c.path || ok != c.ok {
			t.Errorf("Local(%q) = %q, %v; want %q, %v", c.source, path, ok, c.path, c.ok)
		}
	}
	if _, ok := Local("asset:a.webm", ""); ok {
		t.Error("an asset without a root is local")
	}
	if runtime.GOOS == "windows" {
		if path, _ := Local("file:///C:/clips/a.webm", ""); path != `C:\clips\a.webm` {
			t.Errorf("Windows file URL = %q", path)
		}
	}
}

func TestSourceReadsAssets(t *testing.T) {
	root := filepath.Dir(write(t, "vp9a.webm", clips.VP9Alpha))
	if got := Source("asset:/vp9a.webm", root); got != VP9 {
		t.Errorf("asset = %v, want VP9", got)
	}
	if got := Source("https://example.com/vp9a.webm", root); got != None {
		t.Errorf("a remote source is read: %v", got)
	}
}
