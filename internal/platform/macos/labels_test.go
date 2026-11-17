//go:build darwin && !ios

package macos

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func TestNativeLabelTitleRelationship(t *testing.T) {
	if os.Getenv("NSGO_TEST_LABEL_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v", "-test.timeout=30s")
		cmd.Env = append(os.Environ(), "NSGO_TEST_LABEL_PROCESS="+t.Name())
		if out, err := cmd.CombinedOutput(); err != nil || !bytes.Contains(out, []byte("\nPASS\n")) {
			t.Fatalf("native label test (%v):\n%s", err, out)
		}
		return
	}
	onMain(func() {
		loadAppKit()
		s := &surface{views: make([]*view, 3)}
		label := &view{id: 1, kind: driver.KindText, surface: s, native: objc.ID(classView).Send(selNew)}
		s.views[1] = label
		views[label.native] = label
		defer func() { delete(views, label.native); label.native.Send(selRelease) }()
		label.setProps(&driver.ViewProps{Role: driver.RoleText, Label: "Email", LabelFor: 2, LabelPress: true, NoFocus: true})
		label.setEvents(1 << driver.EventPress)
		if role := label.axRole()[0]; role != "AXStaticText" {
			t.Fatalf("label role = %q", role)
		}
		for _, multiline := range []bool{false, true} {
			field := &view{id: 2, kind: driver.KindInput, surface: s, native: objc.ID(classInput).Send(selNew), input: &input{}}
			s.views[2] = field
			views[field.native] = field
			field.setEditor(multiline, false)
			ids := []driver.NodeID{label.id}
			field.setProps(&driver.ViewProps{Role: driver.RoleTextField, Label: "Email", LabelledBy: &ids})
			element := field.accessElement()
			if got := element.Send(sel("accessibilityTitleUIElement")); got != label.native {
				t.Errorf("multiline %v: title element = %v, want %v", multiline, got, label.native)
			}
			if got := element.Send(sel("accessibilityAttributeValue:"), apple.NSString("AXTitleUIElement")); got != label.native {
				t.Errorf("multiline %v: AXTitleUIElement = %v", multiline, got)
			}
			reverse := label.native.Send(sel("accessibilityServesAsTitleForUIElements"))
			if objc.Send[int](reverse, sel("count")) != 1 || reverse.Send(sel("objectAtIndex:"), 0) != element {
				t.Error("label did not expose reverse title relationship")
			}
			field.setProps(&driver.ViewProps{Role: driver.RoleTextField, Label: "Fallback"})
			if element.Send(sel("accessibilityTitleUIElement")) != 0 {
				t.Error("removed relationship remained on editor")
			}
			delete(views, field.native)
			delete(views, field.input.field)
			delete(views, field.input.host)
			field.native.Send(selRelease)
			s.views[2] = nil
		}
	})
}
