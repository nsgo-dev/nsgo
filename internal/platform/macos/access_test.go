//go:build darwin && !ios

package macos

import (
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// TestEditableCombobox checks, through accessibilityAttributeValue: as VoiceOver reads them, that an input with RoleCombobox is an editable combobox on its editor, the field's cell or the text view, with its text, label, value text and the popup's state, its view no element; that a plain input's editor, masked too, takes its label; and that a view with RoleCombobox stays a pop-up button (#246).
func TestEditableCombobox(t *testing.T) { onMain(func() { editableCombobox(t) }) }

func editableCombobox(t *testing.T) {
	loadAppKit()
	attr := func(e objc.ID, name string) string {
		value := e.Send(sel("accessibilityAttributeValue:"), apple.NSString(name))
		if value == 0 {
			return ""
		}
		return apple.GoString(value.Send(sel("description")))
	}
	names := func(e objc.ID) []string {
		var out []string
		list := e.Send(sel("accessibilityAttributeNames"))
		for i := range objc.Send[int](list, sel("count")) {
			out = append(out, apple.GoString(list.Send(sel("objectAtIndex:"), i)))
		}
		return out
	}
	for _, c := range []struct {
		name              string
		multiline, masked bool
		role, plainRole   string
	}{
		{"field", false, false, "AXComboBox", "AXTextField"},
		{"masked field", false, true, "", "AXTextField"},
		{"text view", true, false, "AXComboBox", "AXTextArea"},
	} {
		v := &view{kind: driver.KindInput, native: objc.ID(classInput).Send(selNew), input: &input{}}
		v.setEditor(c.multiline, c.masked)
		v.input.setText("Banana")
		editor := v.input.field
		if !c.multiline {
			editor = editor.Send(sel("cell"))
		}
		plain := attr(editor, "AXRoleDescription")
		v.setProps(&driver.ViewProps{Role: driver.RoleTextField, Label: "Name", TestID: "name"})
		if got := attr(editor, "AXRole"); got != c.plainRole {
			t.Errorf("%s: a plain input's role %q, want %q", c.name, got, c.plainRole)
		}
		if got := attr(editor, "AXDescription"); got != "Name" {
			t.Errorf("%s: a plain input's label %q", c.name, got)
		}
		if got := attr(editor, "AXIdentifier"); got != "name" {
			t.Errorf("%s: a plain input's identifier %q", c.name, got)
		}
		if got := attr(editor, "AXExpanded"); got != "" {
			t.Errorf("%s: a plain input is expanded: %q", c.name, got)
		}
		if !slices.Contains(names(editor), "AXDescription") {
			t.Errorf("%s: a plain input's attributes lack AXDescription: %v", c.name, names(editor))
		}
		if c.role == "" {
			v.native.Send(selRelease)
			delete(views, v.input.field)
			delete(views, v.input.host)
			continue
		}
		v.setProps(&driver.ViewProps{Role: driver.RoleCombobox, Label: "Fruit", TestID: "fruit", Expandable: true, Expanded: true})
		if objc.Send[bool](v.native, sel("isAccessibilityElement")) {
			t.Errorf("%s: a combobox input's view is an element", c.name)
		}
		for name, want := range map[string]string{"AXRole": c.role, "AXDescription": "Fruit", "AXIdentifier": "fruit", "AXExpanded": "1", "AXValue": "Banana"} {
			if got := attr(editor, name); got != want {
				t.Errorf("%s: %s %q, want %q", c.name, name, got, want)
			}
		}
		if got := attr(editor, "AXRoleDescription"); got == "" || got == plain {
			t.Errorf("%s: a combobox's role description %q, a plain input's %q", c.name, got, plain)
		}
		if got := names(editor); !slices.Contains(got, "AXExpanded") || !slices.Contains(got, "AXValue") {
			t.Errorf("%s: a combobox's attributes %v", c.name, got)
		}
		if got := attr(editor, "AXValueDescription"); got != "" {
			t.Errorf("%s: a combobox without a value text describes its value as %q", c.name, got)
		}
		v.setProps(&driver.ViewProps{Role: driver.RoleCombobox, Label: "Fruit", Expandable: true, Value: driver.AccessibilityValue{Text: "Banana"}})
		if got := attr(editor, "AXExpanded"); got != "0" {
			t.Errorf("%s: a closed combobox's AXExpanded %q", c.name, got)
		}
		if got := attr(editor, "AXValueDescription"); got != "" {
			t.Errorf("%s: a value text that is the editor's text is described again as %q", c.name, got)
		}
		// A multiple combobox's chosen items, while its editor holds the query.
		v.setProps(&driver.ViewProps{Role: driver.RoleCombobox, Label: "Fruits", Expandable: true, Value: driver.AccessibilityValue{Text: "Apple, Cherry"}})
		if got := attr(editor, "AXValueDescription"); got != "Apple, Cherry" {
			t.Errorf("%s: a multiple combobox's value description %q, want its chosen items", c.name, got)
		}
		if got := attr(editor, "AXValue"); got != "Banana" {
			t.Errorf("%s: a multiple combobox's value %q, want the editor's text", c.name, got)
		}
		if got := names(editor); !slices.Contains(got, "AXValueDescription") {
			t.Errorf("%s: a described combobox's attributes %v", c.name, got)
		}
		v.native.Send(selRelease)
		delete(views, v.input.field)
		delete(views, v.input.host)
	}

	trigger := &view{id: 1, kind: driver.KindView, native: objc.ID(classView).Send(selNew)}
	views[trigger.native] = trigger
	trigger.setProps(&driver.ViewProps{Role: driver.RoleCombobox, Label: "Kind", Expandable: true})
	// A view outside a window answers accessibilityAttributeValue: with AXUnknown: read its properties.
	if got := apple.GoString(trigger.native.Send(sel("accessibilityRole"))); got != "AXPopUpButton" {
		t.Errorf("a select's trigger is %q, want AXPopUpButton", got)
	}
	if got := apple.GoString(trigger.native.Send(sel("accessibilityLabel"))); got != "Kind" {
		t.Errorf("a select's trigger's label %q", got)
	}
	delete(views, trigger.native)
	trigger.native.Send(selRelease)
}
