package authenticode

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// testKey makes a self-signed code signing certificate and its key, as a
// project's own certificate is.
func testKey(t *testing.T) *Key {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "NSGo Test", Organization: []string{"NSGo"}, Country: []string{"US"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &private.PublicKey, private)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &Key{Private: private, Cert: cert}
}

// testImage is a Windows executable for signing, built as the tool builds
// an app's.
func testImage(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	if err := os.WriteFile(source, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "app.exe")
	cmd := exec.Command("go", "build", "-o", out, source)
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("no Go build for Windows: %v\n%s", err, msg)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSignAppendsASignature(t *testing.T) {
	data := testImage(t)
	if Signed(data) {
		t.Fatal("the image built unsigned reads as signed")
	}
	signed, err := Sign(data, testKey(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !Signed(signed) {
		t.Fatal("the signed image reads as unsigned")
	}
	h, err := parsePE(signed)
	if err != nil {
		t.Fatal(err)
	}
	offset, size := h.certificateTable(signed)
	if size == 0 || offset+size > len(signed) {
		t.Fatalf("certificate table at %d, %d bytes", offset, size)
	}
	if len(signed) < len(data) {
		t.Fatalf("the signed image is shorter than the original: %d < %d", len(signed), len(data))
	}
	// Signing again replaces the signature rather than appending another.
	again, err := Sign(signed, testKey(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	h, err = parsePE(again)
	if err != nil {
		t.Fatal(err)
	}
	_, second := h.certificateTable(again)
	if second == 0 {
		t.Fatal("the image signed twice lost its signature")
	}
	if got := len(again); got > len(signed)+4000 {
		t.Errorf("a second signature grew the image by %d bytes, not replacing the first", got-len(signed))
	}
}

// The digest covers the image as signing leaves it: the checksum and the
// certificate table's entry count as zero, and the table itself is left
// out, so an image signed twice has one digest.
func TestDigestIgnoresSignature(t *testing.T) {
	data := testImage(t)
	h, err := parsePE(data)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := digest(data, h)
	signed, err := Sign(data, testKey(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	h, err = parsePE(signed)
	if err != nil {
		t.Fatal(err)
	}
	if got := digest(imageWithoutSignature(signed, h), h); got != unsigned {
		t.Errorf("the digest changed with the signature:\n%x\n%x", got, unsigned)
	}
}

// An image whose end does not fall on an 8-byte boundary, as NSIS's
// uninstallers' does not, hashes its last bytes padded to one.
func TestDigestPadsTheTail(t *testing.T) {
	data := testImage(t)
	h, err := parsePE(data)
	if err != nil {
		t.Fatal(err)
	}
	// One more byte past the image's end, outside every section.
	padded := append(append([]byte(nil), data...), 0x7f)
	h, err = parsePE(padded)
	if err != nil {
		t.Fatal(err)
	}
	d := sha256.New()
	d.Write(padded[:h.checksum])
	d.Write(padded[h.checksum+4 : h.securityDir])
	d.Write(padded[h.securityDir+8 : h.sizeHeaders])
	end := h.sizeHeaders
	for _, s := range h.sections {
		if s.offset > end {
			d.Write(padded[end:s.offset])
		}
		d.Write(padded[s.offset : s.offset+s.size])
		end = s.offset + s.size
	}
	tail := padded[end:]
	d.Write(tail)
	if len(tail)%8 != 0 {
		d.Write(make([]byte, 8-len(tail)%8))
	}
	var want [sha256.Size]byte
	copy(want[:], d.Sum(nil))
	if got := digest(padded, h); got != want {
		t.Errorf("the tail was not padded:\n%x\n%x", got, want)
	}
}

func TestChecksum(t *testing.T) {
	data := testImage(t)
	h, err := parsePE(data)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := Sign(data, testKey(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	h, err = parsePE(signed)
	if err != nil {
		t.Fatal(err)
	}
	stored := binary.LittleEndian.Uint32(signed[h.checksum:])
	// The same sum, computed over the image with the field read as zero.
	binary.LittleEndian.PutUint32(signed[h.checksum:], 0)
	var sum uint64
	for i := 0; i+1 < len(signed); i += 2 {
		sum += uint64(binary.LittleEndian.Uint16(signed[i:]))
		sum = sum&0xffff + sum>>16
	}
	sum = sum&0xffff + sum>>16
	if got := uint32(sum) + uint32(len(signed)); got != stored {
		t.Errorf("checksum %#x, computed %#x", stored, got)
	}
}

func TestSignRefusesOtherFiles(t *testing.T) {
	for _, data := range [][]byte{
		nil,
		[]byte("not a program at all, but a text file of some length"),
		[]byte("MZ"),
	} {
		if _, err := Sign(data, testKey(t), nil); !errors.Is(err, ErrNotPE) {
			t.Errorf("Sign(%q) = %v, want ErrNotPE", data, err)
		}
	}
}

func TestSignRefusesAnotherKey(t *testing.T) {
	key := testKey(t)
	other := testKey(t)
	key.Cert = other.Cert
	if _, err := Sign(testImage(t), key, nil); err == nil {
		t.Error("a certificate that is not the key's signs")
	}
}

// pemCert writes cert as PEM, for osslsigncode to trust.
func pemCert(t *testing.T, key *Key) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cert.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: key.Cert.Raw})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// osslsigncode checks the signature as Windows and the other tools do,
// where it is installed.
func TestSignVerifiesWithOsslsigncode(t *testing.T) {
	tool, err := exec.LookPath("osslsigncode")
	if err != nil {
		t.Skip("no osslsigncode")
	}
	key := testKey(t)
	signed, err := Sign(testImage(t), key, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "signed.exe")
	if err := os.WriteFile(path, signed, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(tool, "verify", "-in", path, "-CAfile", pemCert(t, key)).CombinedOutput()
	if err != nil {
		t.Fatalf("osslsigncode verify: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("Signature verification: ok")) {
		t.Errorf("osslsigncode verify:\n%s", out)
	}
}
