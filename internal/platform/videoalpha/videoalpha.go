// Package videoalpha reads whether a video file declares an alpha channel, for the backends whose players do not say so before they draw: VP8 and VP9 in Matroska or WebM with the track's AlphaMode set, and HEVC whose decoder configuration carries the alpha channel information SEI, as Apple's HEVC with alpha has it. It reads a local file's container headers alone, never its frames, within a bound, and imports nothing of the module.
package videoalpha

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Codec is the codec of a video track that declares an alpha channel, or None.
type Codec uint8

const (
	None Codec = iota
	// VP8 is VP8 with alpha in Matroska or WebM, its alpha coded as a second stream in each block's additions.
	VP8
	// VP9 is VP9 with alpha in Matroska or WebM, as VP8's.
	VP9
	// HEVC is HEVC with an alpha layer in a QuickTime or MPEG-4 file, as Apple's encoders write it.
	HEVC
)

// String names a codec with alpha for messages.
func (c Codec) String() string {
	switch c {
	case VP8:
		return "VP8 with alpha"
	case VP9:
		return "VP9 with alpha"
	case HEVC:
		return "HEVC with alpha"
	}
	return "none"
}

// maxHeader is the most bytes of one container element or box read whole: a file's track headers are a few kilobytes.
const maxHeader = 1 << 20

// Source returns the alpha codec of a Video source: an "asset:" path under root, a file URL or a file path. Other URLs are not read and give None.
func Source(source, root string) Codec {
	path, ok := Local(source, root)
	if !ok {
		return None
	}
	return File(path)
}

// Local returns the file a Video source names: an "asset:" path under root, a file URL or a file path. ok is false for other URLs and for an asset without a root.
func Local(source, root string) (path string, ok bool) {
	if p, isAsset := strings.CutPrefix(source, "asset:"); isAsset {
		if root == "" {
			return "", false
		}
		return filepath.Join(root, filepath.FromSlash(strings.TrimLeft(p, "/"))), true
	}
	if !strings.Contains(source, "://") {
		return source, source != ""
	}
	u, err := url.Parse(source)
	if err != nil || !strings.EqualFold(u.Scheme, "file") || u.Path == "" {
		return "", false
	}
	path = u.Path
	// file:///C:/clip.webm is C:\clip.webm on Windows.
	if runtime.GOOS == "windows" && len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path), true
}

// File returns the alpha codec of the video file at path, None when it declares none or cannot be read.
func File(path string) Codec {
	f, err := os.Open(path)
	if err != nil {
		return None
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return None
	}
	return Read(f, info.Size())
}

// Read returns the alpha codec of a video file of size bytes read through r.
func Read(r io.ReaderAt, size int64) Codec {
	var head [12]byte
	if n, _ := r.ReadAt(head[:], 0); n < 8 {
		return None
	}
	if binary.BigEndian.Uint32(head[:4]) == idEBML {
		return matroska(r, size)
	}
	return mpeg4(r, size)
}

// Matroska's element IDs, with their length markers.
const (
	idEBML       = 0x1a45dfa3
	idSegment    = 0x18538067
	idTracks     = 0x1654ae6b
	idCluster    = 0x1f43b675
	idTrackEntry = 0xae
	idTrackType  = 0x83
	idCodecID    = 0x86
	idVideo      = 0xe0
	idAlphaMode  = 0x53c0
	trackVideo   = 1
)

// unknownSize is an element whose size is not written, as a live stream's segment.
const unknownSize = -1

// element reads the header of the EBML element at off: its ID, its data's offset and its size, unknownSize when not written.
func element(r io.ReaderAt, off int64) (id uint32, data, size int64, ok bool) {
	var b [12]byte
	n, _ := r.ReadAt(b[:], off)
	idLen := vintLen(b[0])
	if idLen == 0 || idLen > 4 || n < idLen+1 {
		return 0, 0, 0, false
	}
	for _, c := range b[:idLen] {
		id = id<<8 | uint32(c)
	}
	sizeLen := vintLen(b[idLen])
	if sizeLen == 0 || n < idLen+sizeLen {
		return 0, 0, 0, false
	}
	v := uint64(b[idLen]) & (0xff >> sizeLen)
	all := v == 0xff>>sizeLen
	for _, c := range b[idLen+1 : idLen+sizeLen] {
		v = v<<8 | uint64(c)
		all = all && c == 0xff
	}
	size = int64(v)
	if all {
		size = unknownSize
	} else if v > 1<<62 {
		return 0, 0, 0, false
	}
	return id, off + int64(idLen+sizeLen), size, true
}

// vintLen is the length of the EBML variable-size integer whose first byte is b, 0 when b starts none.
func vintLen(b byte) int {
	for n := 1; n <= 8; n++ {
		if b&(0x80>>(n-1)) != 0 {
			return n
		}
	}
	return 0
}

// matroska finds the Tracks of a Matroska or WebM file's segment, which come before its first cluster.
func matroska(r io.ReaderAt, size int64) Codec {
	_, data, n, ok := element(r, 0)
	if !ok || n < 0 {
		return None
	}
	id, data, n, ok := element(r, data+n)
	if !ok || id != idSegment {
		return None
	}
	end := size
	if n != unknownSize {
		end = min(end, data+n)
	}
	for off, i := data, 0; off < end && i < 256; i++ {
		id, data, n, ok := element(r, off)
		if !ok || n == unknownSize || id == idCluster {
			return None
		}
		if id == idTracks {
			if n > maxHeader {
				return None
			}
			b := make([]byte, n)
			if _, err := r.ReadAt(b, data); err != nil {
				return None
			}
			return tracks(b)
		}
		off = data + n
	}
	return None
}

// children calls fn with each child element of the element data b holds, until fn returns false.
func children(b []byte, fn func(id uint32, data []byte) bool) {
	r := bytes.NewReader(b)
	for off := int64(0); off < int64(len(b)); {
		id, data, n, ok := element(r, off)
		if !ok || n == unknownSize || data+n > int64(len(b)) {
			return
		}
		if !fn(id, b[data:data+n]) {
			return
		}
		off = data + n
	}
}

// uintOf is the value of an EBML unsigned integer element's data.
func uintOf(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

// tracks returns the alpha codec of the first video track of a Tracks element's data that declares alpha.
func tracks(b []byte) Codec {
	found := None
	children(b, func(id uint32, entry []byte) bool {
		if id != idTrackEntry {
			return true
		}
		var kind, alpha uint64
		var codec string
		children(entry, func(id uint32, data []byte) bool {
			switch id {
			case idTrackType:
				kind = uintOf(data)
			case idCodecID:
				codec = strings.TrimRight(string(data), "\x00")
			case idVideo:
				children(data, func(id uint32, data []byte) bool {
					if id == idAlphaMode {
						alpha = uintOf(data)
					}
					return true
				})
			}
			return true
		})
		if kind == trackVideo && alpha == 1 {
			switch codec {
			case "V_VP8":
				found = VP8
			case "V_VP9":
				found = VP9
			}
		}
		return found == None
	})
	return found
}

// box reads the header of the ISO base media box at off: its type, its data's offset and its size.
func box(r io.ReaderAt, off, end int64) (kind string, data, size int64, ok bool) {
	var b [16]byte
	if n, _ := r.ReadAt(b[:8], off); n < 8 {
		return "", 0, 0, false
	}
	n, kind, data := int64(binary.BigEndian.Uint32(b[:4])), string(b[4:8]), off+8
	switch n {
	case 0:
		n = end - off
	case 1:
		if got, _ := r.ReadAt(b[8:16], off+8); got < 8 {
			return "", 0, 0, false
		}
		n, data = int64(binary.BigEndian.Uint64(b[8:16])), off+16
	}
	// n > end-off rather than off+n > end: a 64-bit size near the largest int64 would overflow the sum.
	if n < data-off || n > end-off {
		return "", 0, 0, false
	}
	return kind, data, off + n - data, true
}

// moovPath is the chain of boxes from the top of a QuickTime or MPEG-4 file down to a track's sample descriptions.
var moovPath = [...]string{"moov", "trak", "mdia", "minf", "stbl", "stsd"}

// maxBoxes is the most box headers mpeg4 reads in one file.
const maxBoxes = 4096

// mpeg4 finds the movie box of a QuickTime or MPEG-4 file, at its start or its end, and walks its tracks down to their sample descriptions, reading those alone: a long movie's sample tables run to megabytes.
func mpeg4(r io.ReaderAt, size int64) Codec {
	found, read := None, 0
	// walk looks through the boxes in [off, end) at depth along moovPath.
	var walk func(off, end int64, depth int)
	walk = func(off, end int64, depth int) {
		for off < end && found == None && read < maxBoxes {
			read++
			kind, data, n, ok := box(r, off, end)
			if !ok {
				return
			}
			if kind == moovPath[depth] {
				if depth < len(moovPath)-1 {
					walk(data, data+n, depth+1)
				} else if n <= maxHeader {
					b := make([]byte, n)
					if _, err := r.ReadAt(b, data); err == nil {
						found = sampleDescriptions(b)
					}
				}
				if depth == 0 {
					// A file has one movie box.
					return
				}
			}
			off = data + n
		}
	}
	walk(0, size, 0)
	return found
}

// boxes calls fn with each box b holds, until fn returns false.
func boxes(b []byte, fn func(kind string, data []byte) bool) {
	r := bytes.NewReader(b)
	for off := int64(0); off < int64(len(b)); {
		kind, data, n, ok := box(r, off, int64(len(b)))
		if !ok || !fn(kind, b[data:data+n]) {
			return
		}
		off = data + n
	}
}

// sampleDescriptions returns HEVC when an stsd box's data has an hvc1 or hev1 entry whose hvcC carries the alpha channel information SEI, None otherwise.
func sampleDescriptions(b []byte) Codec {
	// A full box's version and flags, and the count of entries.
	if len(b) <= 8 {
		return None
	}
	found := None
	boxes(b[8:], func(kind string, entry []byte) bool {
		if (kind == "hvc1" || kind == "hev1") && hevcAlpha(entry) {
			found = HEVC
		}
		return found == None
	})
	return found
}

// visualSampleEntry is the size of a visual sample entry's fields, before its boxes.
const visualSampleEntry = 78

// hevcAlpha reports whether an hvc1 or hev1 sample entry's decoder configuration carries the alpha channel information SEI.
func hevcAlpha(entry []byte) bool {
	if len(entry) < visualSampleEntry {
		return false
	}
	alpha := false
	boxes(entry[visualSampleEntry:], func(kind string, c []byte) bool {
		if kind == "hvcC" {
			alpha = hvcCAlpha(c)
		}
		return kind != "hvcC"
	})
	return alpha
}

const (
	nalPrefixSEI     = 39
	seiAlphaChannels = 165 // alpha_channel_info
)

// hvcCAlpha reports whether an HEVC decoder configuration record holds a prefix SEI with alpha channel information.
func hvcCAlpha(c []byte) bool {
	const arrays = 22
	if len(c) <= arrays {
		return false
	}
	p := arrays + 1
	for range int(c[arrays]) {
		if p+3 > len(c) {
			return false
		}
		kind, count := c[p]&0x3f, int(binary.BigEndian.Uint16(c[p+1:]))
		p += 3
		for range count {
			if p+2 > len(c) {
				return false
			}
			n := int(binary.BigEndian.Uint16(c[p:]))
			p += 2
			if p+n > len(c) {
				return false
			}
			if kind == nalPrefixSEI && seiHas(c[p:p+n], seiAlphaChannels) {
				return true
			}
			p += n
		}
	}
	return false
}

// seiHas reports whether an SEI NAL unit holds a message of payload type want.
func seiHas(nal []byte, want int) bool {
	if len(nal) < 2 {
		return false
	}
	// The payload without its emulation prevention bytes, 0x000003 coded for 0x0000.
	rbsp := make([]byte, 0, len(nal)-2)
	zeros := 0
	for _, c := range nal[2:] {
		if zeros >= 2 && c == 3 {
			zeros = 0
			continue
		}
		if c == 0 {
			zeros++
		} else {
			zeros = 0
		}
		rbsp = append(rbsp, c)
	}
	// Each message is its type and size, each coded as runs of 0xff and a last byte, then its payload; the trailing bits start with 0x80.
	for p := 0; p < len(rbsp) && rbsp[p] != 0x80; {
		kind, size := 0, 0
		for p < len(rbsp) && rbsp[p] == 0xff {
			kind += 255
			p++
		}
		if p >= len(rbsp) {
			return false
		}
		kind += int(rbsp[p])
		p++
		for p < len(rbsp) && rbsp[p] == 0xff {
			size += 255
			p++
		}
		if p >= len(rbsp) {
			return false
		}
		size += int(rbsp[p])
		p++
		if kind == want {
			return true
		}
		p += size
	}
	return false
}
