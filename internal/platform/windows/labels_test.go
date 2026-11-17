//go:build windows

package windows

import (
	"testing"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestNativeLabeledByProvider(t *testing.T) {
	ids := []driver.NodeID{1}
	label := &view{id: 1, kind: driver.KindText, props: driver.ViewProps{Role: driver.RoleText, LabelPress: true, NoFocus: true}, events: 1 << driver.EventPress}
	field := &view{id: 2, props: driver.ViewProps{Role: driver.RoleTextField, Label: "Email", LabelledBy: &ids}}
	w := &window{views: []*view{nil, label, field}}
	provider := w.element(2, 0)
	var value variant
	if code := uiaGetPropertyValue(&provider.simple, propLabeledBy, &value); code != sOK || value.vt != vtUnknown {
		t.Fatalf("LabeledBy = %+v, code %x", value, code)
	}
	want := &w.element(1, 0).simple
	if value.val != uint64(uintptr(unsafe.Pointer(want))) {
		t.Fatal("LabeledBy did not return the label's provider")
	}
	uiaRelease(want)
	if s := w.element(1, 0).spec(label); s.control != ctText || s.focusable {
		t.Fatalf("label semantics = %+v", s)
	}
	field.props.LabelledBy = nil
	uiaGetPropertyValue(&provider.simple, propLabeledBy, &value)
	if value.vt != vtEmpty {
		t.Fatal("removed relationship remained")
	}
}
