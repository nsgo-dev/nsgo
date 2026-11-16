package authenticode

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

// A PE image is signed by appending a WIN_CERTIFICATE that holds a PKCS #7
// SignedData, whose content states the image's digest. The digest covers
// the image as signing reads it, its checksum field and its certificate
// table's entry left zero and the table itself left out, and Windows
// recomputes it over everything the signature does not cover.

const (
	// winCertificateRevision2 is WIN_CERT_REVISION_2_0, which every
	// signature since Windows XP carries.
	winCertificateRevision2 = 0x0200
	// winCertificateTypePKCS is WIN_CERT_TYPE_PKCS_SIGNED_DATA.
	winCertificateTypePKCS = 0x0002
	// certificateAlignment is the boundary the table starts on and its
	// entries end at.
	certificateAlignment = 8
)

// ErrNotPE is what Sign returns for a file that is no PE image.
var ErrNotPE = errors.New("not a PE image")

// peHeader locates what signing reads and changes.
type peHeader struct {
	checksum    int // the optional header's CheckSum field
	securityDir int // the certificate table's directory entry
	sizeHeaders int // SizeOfHeaders, where the sections start
	sections    []peSection
}

type peSection struct {
	offset, size int
}

// parsePE reads an image's headers, refusing one whose parts do not fit it.
func parsePE(data []byte) (*peHeader, error) {
	if len(data) < 0x40 || string(data[:2]) != "MZ" {
		return nil, ErrNotPE
	}
	pe := int(binary.LittleEndian.Uint32(data[0x3c:]))
	if pe < 0x40 || pe+24 > len(data) || string(data[pe:pe+4]) != "PE\x00\x00" {
		return nil, ErrNotPE
	}
	numberOfSections := int(binary.LittleEndian.Uint16(data[pe+6:]))
	sizeOfOptional := int(binary.LittleEndian.Uint16(data[pe+20:]))
	optional := pe + 24
	if optional+sizeOfOptional > len(data) {
		return nil, errors.New("the optional header runs past the image")
	}
	var directories int
	switch binary.LittleEndian.Uint16(data[optional:]) {
	case 0x10b: // PE32
		directories = 96
	case 0x20b: // PE32+
		directories = 112
	default:
		return nil, fmt.Errorf("unknown optional header magic %#x", binary.LittleEndian.Uint16(data[optional:]))
	}
	if sizeOfOptional < directories+5*8 {
		return nil, errors.New("the optional header holds no certificate table entry")
	}
	h := &peHeader{
		checksum:    optional + 64,
		securityDir: optional + directories + 4*8,
		sizeHeaders: int(binary.LittleEndian.Uint32(data[optional+60:])),
	}
	table := optional + sizeOfOptional
	if table+numberOfSections*40 > len(data) {
		return nil, errors.New("the section table runs past the image")
	}
	for i := range numberOfSections {
		s := table + i*40
		size := int(binary.LittleEndian.Uint32(data[s+16:]))
		offset := int(binary.LittleEndian.Uint32(data[s+20:]))
		if size == 0 {
			continue
		}
		if offset < 0 || offset+size > len(data) {
			return nil, errors.New("a section runs past the image")
		}
		h.sections = append(h.sections, peSection{offset, size})
	}
	sort.Slice(h.sections, func(i, j int) bool { return h.sections[i].offset < h.sections[j].offset })
	if h.sizeHeaders <= 0 || h.sizeHeaders > len(data) {
		return nil, errors.New("the headers' size is not the image's")
	}
	return h, nil
}

// certificateTable is the image's table as its directory names it: where
// it starts and how long it is, both zero when the image carries none.
func (h *peHeader) certificateTable(data []byte) (offset, size int) {
	offset = int(binary.LittleEndian.Uint32(data[h.securityDir:]))
	size = int(binary.LittleEndian.Uint32(data[h.securityDir+4:]))
	if offset <= 0 || size <= 0 || offset+size > len(data) {
		return 0, 0
	}
	return offset, size
}

// Signed reports whether a PE image carries a signature.
func Signed(data []byte) bool {
	h, err := parsePE(data)
	if err != nil {
		return false
	}
	_, size := h.certificateTable(data)
	return size > 0
}

// imageWithoutSignature is the image with its signature, if it had one,
// removed, so that signing it again replaces that signature rather than
// appending a second.
func imageWithoutSignature(data []byte, h *peHeader) []byte {
	offset, _ := h.certificateTable(data)
	if offset == 0 {
		return data
	}
	out := append([]byte(nil), data[:offset]...)
	binary.LittleEndian.PutUint32(out[h.securityDir:], 0)
	binary.LittleEndian.PutUint32(out[h.securityDir+4:], 0)
	return out
}

// digest is the Authenticode digest of an image that holds no signature:
// its headers without the checksum and the certificate table's entry, each
// section's data in the file's order, and what the sections leave out, the
// space a signature took included. An image whose end does not fall on an
// 8-byte boundary, as NSIS's uninstallers' does not, pads its last bytes
// to one.
func digest(data []byte, h *peHeader) [sha256.Size]byte {
	d := sha256.New()
	d.Write(data[:h.checksum])
	d.Write(data[h.checksum+4 : h.securityDir])
	d.Write(data[h.securityDir+8 : h.sizeHeaders])
	end := h.sizeHeaders
	for _, s := range h.sections {
		if s.offset > end {
			d.Write(data[end:s.offset])
		}
		d.Write(data[s.offset : s.offset+s.size])
		end = s.offset + s.size
	}
	if end < len(data) {
		tail := data[end:]
		d.Write(tail)
		d.Write(zeroes[:(8-len(tail)%8)%8])
	}
	var sum [sha256.Size]byte
	copy(sum[:], d.Sum(nil))
	return sum
}

var zeroes [8]byte

// addCertificate appends table, a signature's DER, to the image as its
// certificate table, and writes the entry that names it.
func addCertificate(data []byte, h *peHeader, table []byte) []byte {
	out := append([]byte(nil), data...)
	for len(out)%certificateAlignment != 0 {
		out = append(out, 0)
	}
	offset := len(out)
	total := (8 + len(table) + certificateAlignment - 1) / certificateAlignment * certificateAlignment
	out = binary.LittleEndian.AppendUint32(out, uint32(total))
	out = binary.LittleEndian.AppendUint16(out, winCertificateRevision2)
	out = binary.LittleEndian.AppendUint16(out, winCertificateTypePKCS)
	out = append(out, table...)
	for len(out)-offset < total {
		out = append(out, 0)
	}
	binary.LittleEndian.PutUint32(out[h.securityDir:], uint32(offset))
	binary.LittleEndian.PutUint32(out[h.securityDir+4:], uint32(total))
	return out
}

// setChecksum writes the checksum Windows computes: the image's 16-bit
// words added with a carry, the checksum field itself counting as zero,
// and the file's length added.
func setChecksum(data []byte, h *peHeader) {
	binary.LittleEndian.PutUint32(data[h.checksum:], 0)
	var sum uint64
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint64(binary.LittleEndian.Uint16(data[i:]))
		sum = sum&0xffff + sum>>16
	}
	if len(data)%2 == 1 {
		sum += uint64(data[len(data)-1])
		sum = sum&0xffff + sum>>16
	}
	sum = sum&0xffff + sum>>16
	sum += uint64(len(data))
	binary.LittleEndian.PutUint32(data[h.checksum:], uint32(sum))
}

// Sign signs a PE image with key and, with a stamper, a timestamp, and
// returns the image with its signature. An image signed before is signed
// again in place of its old signature.
func Sign(data []byte, key *Key, stamp Stamper) ([]byte, error) {
	h, err := parsePE(data)
	if err != nil {
		return nil, err
	}
	unsigned := imageWithoutSignature(data, h)
	if h, err = parsePE(unsigned); err != nil {
		return nil, err
	}
	table, err := signedData(digest(unsigned, h), key, stamp)
	if err != nil {
		return nil, err
	}
	signed := addCertificate(unsigned, h, table)
	setChecksum(signed, h)
	return signed, nil
}
