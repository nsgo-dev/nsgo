//go:build darwin

package apple

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestMetadataTypesOfAScan(t *testing.T) {
	if got := MetadataTypes(driver.ScanRequest{}); !slices.Equal(got, []string{"org.iso.QRCode"}) {
		t.Errorf("a default scan reads %v, want a QR code alone", got)
	}
	got := MetadataTypes(driver.ScanRequest{Kinds: []driver.CodeKind{driver.CodeEAN13, driver.CodeITF}})
	if !slices.Equal(got, []string{"org.gs1.EAN-13", "org.ansi.Interleaved2of5"}) {
		t.Errorf("EAN-13 and ITF read %v", got)
	}
	for k := driver.CodeQR; k <= driver.CodeITF; k++ {
		if back, ok := MetadataKind(metadataTypes[k]); !ok || back != k {
			t.Errorf("kind %d does not read back from %q", k, metadataTypes[k])
		}
	}
	if _, ok := MetadataKind("face"); ok {
		t.Error("a face reads as a code")
	}
}
