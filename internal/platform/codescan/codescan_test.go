package codescan

import (
	"image"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// frame is a grey camera frame of width by height with img drawn at x, y, as luminance.
func frame(img image.Image, width, height, x, y int) []byte {
	lum := make([]byte, width*height)
	for i := range lum {
		lum[i] = 140
	}
	b := img.Bounds()
	for py := b.Min.Y; py < b.Max.Y; py++ {
		for px := b.Min.X; px < b.Max.X; px++ {
			r, _, _, _ := img.At(px, py).RGBA()
			lum[(y+py-b.Min.Y)*width+x+px-b.Min.X] = byte(r >> 8)
		}
	}
	return lum
}

func TestDecodeReadsAQRCodeInAFrame(t *testing.T) {
	code, err := qrcode.NewQRCodeWriter().Encode("myapp://pair?c=123456", gozxing.BarcodeFormat_QR_CODE, 240, 240, nil)
	if err != nil {
		t.Fatal(err)
	}
	lum := frame(code, 640, 480, 200, 120)
	got := Decode(lum, 640, 480, driver.ScanRequest{})
	if got == nil || got.Kind != driver.CodeQR || got.Text != "myapp://pair?c=123456" {
		t.Fatalf("Decode = %+v", got)
	}
	if got := Decode(lum, 640, 480, driver.ScanRequest{Kinds: []driver.CodeKind{driver.CodeEAN13}}); got != nil {
		t.Errorf("a scan of EAN-13 read the QR code: %+v", got)
	}
}

func TestDecodeReadsABarcodeEitherWay(t *testing.T) {
	code, err := oned.NewEAN13Writer().Encode("4006381333931", gozxing.BarcodeFormat_EAN_13, 380, 120, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := driver.ScanRequest{Kinds: []driver.CodeKind{driver.CodeQR, driver.CodeEAN13}}
	lum := frame(code, 640, 480, 130, 180)
	if got := Decode(lum, 640, 480, r); got == nil || got.Kind != driver.CodeEAN13 || got.Text != "4006381333931" {
		t.Fatalf("Decode of a level barcode = %+v", got)
	}
	// A phone's sensor sees a barcode held level across its frame on end.
	if got := Decode(turn(lum, 640, 480), 480, 640, r); got == nil || got.Text != "4006381333931" {
		t.Fatalf("Decode of a barcode on end = %+v", got)
	}
}

func TestDecodeFindsNothingInAnEmptyFrame(t *testing.T) {
	if got := Decode(make([]byte, 320*240), 320, 240, driver.ScanRequest{}); got != nil {
		t.Errorf("an empty frame read %+v", got)
	}
	if got := Decode(make([]byte, 10), 320, 240, driver.ScanRequest{}); got != nil {
		t.Errorf("a short frame read %+v", got)
	}
	if Reads(driver.CodePDF417) || !Reads(driver.CodeITF) {
		t.Error("Reads names the wrong kinds")
	}
}

func TestTurnTurnsAQuarterClockwise(t *testing.T) {
	// 1 2 3      4 1
	// 4 5 6  ->  5 2
	//            6 3
	got := turn([]byte{1, 2, 3, 4, 5, 6}, 3, 2)
	want := []byte{4, 1, 5, 2, 6, 3}
	if string(got) != string(want) {
		t.Errorf("turn = %v, want %v", got, want)
	}
}
