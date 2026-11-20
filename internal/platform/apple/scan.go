//go:build darwin

package apple

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// metadataTypes are AVFoundation's AVMetadataObjectType values of each kind of code driver.ScanRequest asks for; an UPC-A code reads as an EAN-13 with a leading 0.
var metadataTypes = map[driver.CodeKind]string{
	driver.CodeQR:         "org.iso.QRCode",
	driver.CodeAztec:      "org.iso.Aztec",
	driver.CodeDataMatrix: "org.iso.DataMatrix",
	driver.CodePDF417:     "org.iso.PDF417",
	driver.CodeEAN13:      "org.gs1.EAN-13",
	driver.CodeEAN8:       "org.gs1.EAN-8",
	driver.CodeUPCE:       "org.gs1.UPC-E",
	driver.CodeCode128:    "org.iso.Code128",
	driver.CodeCode39:     "org.iso.Code39",
	driver.CodeCode93:     "com.intermec.Code93",
	driver.CodeITF:        "org.ansi.Interleaved2of5",
}

// MetadataTypes are the AVMetadataObjectTypes r reads.
func MetadataTypes(r driver.ScanRequest) []string {
	var types []string
	for k := driver.CodeQR; k <= driver.CodeITF; k++ {
		if r.Wants(k) {
			types = append(types, metadataTypes[k])
		}
	}
	return types
}

// MetadataKind is the kind of code of an AVMetadataObjectType, and whether it is one.
func MetadataKind(t string) (driver.CodeKind, bool) {
	for k, v := range metadataTypes {
		if v == t {
			return k, true
		}
	}
	return 0, false
}

// MainQueue is the main dispatch queue, as AVFoundation's delegates take it.
func MainQueue() objc.ID { return objc.ID(mainQueue) }
