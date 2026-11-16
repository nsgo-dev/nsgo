//go:build darwin && !ios

package macos

import (
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// axAttr is an element's attribute as VoiceOver reads it, through accessibilityAttributeValue:, described.
func axAttr(e objc.ID, name string) string {
	value := e.Send(sel("accessibilityAttributeValue:"), apple.NSString(name))
	if value == 0 {
		return ""
	}
	return apple.GoString(value.Send(sel("description")))
}

func axAttrNames(e objc.ID) []string { return axList(e.Send(sel("accessibilityAttributeNames"))) }

func axActionNames(e objc.ID) []string { return axList(e.Send(sel("accessibilityActionNames"))) }

func axList(list objc.ID) []string {
	var out []string
	for i := range objc.Send[int](list, sel("count")) {
		out = append(out, apple.GoString(list.Send(sel("objectAtIndex:"), i)))
	}
	return out
}

// TestEditableCombobox checks, through accessibilityAttributeValue: as VoiceOver reads them, that an input with RoleCombobox is an editable combobox on its editor, the field's cell or the text view, with its text, label, value text and the popup's state, its view no element; that a plain input's editor, masked too, takes its label; and that a view with RoleCombobox stays a pop-up button (#246).
func TestEditableCombobox(t *testing.T) { onMain(func() { editableCombobox(t) }) }

func editableCombobox(t *testing.T) {
	loadAppKit()
	attr, names := axAttr, axAttrNames
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

// actionEngine is a platform that shows nothing, for an engine that only hears the events a test's views report.
type actionEngine struct{ apple.TextEngine }

func (actionEngine) Mount(*driver.MountBatch) {}
func (actionEngine) RequestFrame()            {}

// TestEditableSpinButton checks, through the accessibility API VoiceOver uses, that an input with RoleSpinButton is an editable incrementor on its editor, the field's cell or the text view, with its text, label and test ID, whose AXIncrement and AXDecrement report the input's actions, its view no element; that an input without the actions offers none; and that a view with RoleSpinButton stays an incrementor of its own (#254).
func TestEditableSpinButton(t *testing.T) { onMain(func() { editableSpinButton(t) }) }

func editableSpinButton(t *testing.T) {
	loadAppKit()
	var reported []driver.Event
	eng := engine.Start(actionEngine{}, func(*core.Scope) core.Node { return core.View{} }, engine.Options{Event: func(e driver.Event) {
		if e.Kind == driver.EventAccessibilityAction {
			reported = append(reported, e)
		}
	}})
	defer eng.Close()
	s := &surface{engine: eng}
	s.animator = driver.NewAnimator(func(driver.Event) {})
	actions := &[]driver.AccessibilityAction{{Name: driver.ActionIncrement}, {Name: driver.ActionDecrement}}
	for _, c := range []struct {
		name      string
		multiline bool
		plainRole string
	}{
		{"field", false, "AXTextField"},
		{"text view", true, "AXTextArea"},
	} {
		v := &view{id: 7, kind: driver.KindInput, surface: s, native: objc.ID(classInput).Send(selNew), input: &input{}}
		v.events = 1 << driver.EventAccessibilityAction
		v.setEditor(c.multiline, false)
		v.input.setText("12")
		editor := v.input.field
		if !c.multiline {
			editor = editor.Send(sel("cell"))
		}
		plain := axAttr(editor, "AXRoleDescription")
		v.setProps(&driver.ViewProps{Role: driver.RoleTextField, Label: "Count"})
		if got := axActionNames(editor); slices.Contains(got, "AXIncrement") || slices.Contains(got, "AXDecrement") {
			t.Errorf("%s: a plain input offers %v", c.name, got)
		}
		if got := axAttr(editor, "AXRole"); got != c.plainRole {
			t.Errorf("%s: a plain input's role %q, want %q", c.name, got, c.plainRole)
		}

		v.setProps(&driver.ViewProps{Role: driver.RoleSpinButton, Label: "Count", TestID: "count", Actions: actions,
			Value: driver.AccessibilityValue{Min: 0, Max: 20, Now: 12, Text: "12"}})
		if objc.Send[bool](v.native, sel("isAccessibilityElement")) {
			t.Errorf("%s: a spin button input's view is an element", c.name)
		}
		for name, want := range map[string]string{"AXRole": "AXIncrementor", "AXDescription": "Count", "AXIdentifier": "count", "AXValue": "12", "AXValueDescription": ""} {
			if got := axAttr(editor, name); got != want {
				t.Errorf("%s: %s %q, want %q", c.name, name, got, want)
			}
		}
		if got := axAttr(editor, "AXRoleDescription"); got == "" || got == plain {
			t.Errorf("%s: a spin button's role description %q, a plain input's %q", c.name, got, plain)
		}
		got := axActionNames(editor)
		if !slices.Contains(got, "AXIncrement") || !slices.Contains(got, "AXDecrement") || slices.Contains(got, "AXCancel") {
			t.Errorf("%s: a spin button's actions %v", c.name, got)
		}
		if d := apple.GoString(editor.Send(sel("accessibilityActionDescription:"), apple.NSString("AXIncrement"))); d == "" {
			t.Errorf("%s: AXIncrement has no description", c.name)
		}
		reported = nil
		editor.Send(sel("accessibilityPerformAction:"), apple.NSString("AXIncrement"))
		editor.Send(sel("accessibilityPerformAction:"), apple.NSString("AXDecrement"))
		editor.Send(sel("accessibilityPerformAction:"), apple.NSString("AXCancel"))
		var names []string
		for _, e := range reported {
			if e.ID != v.id {
				t.Errorf("%s: an action reported for view %d", c.name, e.ID)
			}
			names = append(names, e.Text)
		}
		if want := []string{driver.ActionIncrement, driver.ActionDecrement}; !slices.Equal(names, want) {
			t.Errorf("%s: performing AXIncrement, AXDecrement and AXCancel reported %v, want %v", c.name, names, want)
		}
		v.native.Send(selRelease)
		delete(views, v.input.field)
		delete(views, v.input.host)
	}

	stepper := &view{id: 1, kind: driver.KindView, native: objc.ID(classView).Send(selNew)}
	views[stepper.native] = stepper
	stepper.setProps(&driver.ViewProps{Role: driver.RoleSpinButton, Label: "Count", Actions: actions})
	if got := apple.GoString(stepper.native.Send(sel("accessibilityRole"))); got != "AXIncrementor" {
		t.Errorf("a spin button view is %q, want AXIncrementor", got)
	}
	if !objc.Send[bool](stepper.native, sel("isAccessibilityElement")) {
		t.Error("a spin button view is no element")
	}
	delete(views, stepper.native)
	stepper.native.Send(selRelease)
}

// TestSearchFieldAndStatus checks, through the accessibility API VoiceOver uses, that an input with RoleSearchField stays an editable text field on its editor, with AXSearchField's subrole and role description, its view no element, and that a view with RoleStatus is a group with AXApplicationStatus's subrole whose children stay elements.
func TestSearchFieldAndStatus(t *testing.T) { onMain(func() { searchFieldAndStatus(t) }) }

func searchFieldAndStatus(t *testing.T) {
	loadAppKit()
	v := &view{kind: driver.KindInput, native: objc.ID(classInput).Send(selNew), input: &input{}}
	v.setEditor(false, false)
	v.input.setText("query")
	editor := v.input.field.Send(sel("cell"))
	v.setProps(&driver.ViewProps{Role: driver.RoleTextField, Label: "Find"})
	plain := axAttr(editor, "AXRoleDescription")
	v.setProps(&driver.ViewProps{Role: driver.RoleSearchField, Label: "Find", TestID: "find"})
	if objc.Send[bool](v.native, sel("isAccessibilityElement")) {
		t.Error("a search field's view is an element")
	}
	for name, want := range map[string]string{"AXRole": "AXTextField", "AXSubrole": "AXSearchField", "AXDescription": "Find", "AXIdentifier": "find", "AXValue": "query"} {
		if got := axAttr(editor, name); got != want {
			t.Errorf("a search field's %s %q, want %q", name, got, want)
		}
	}
	if got := axAttr(editor, "AXRoleDescription"); got == "" || got == plain {
		t.Errorf("a search field's role description %q, a plain input's %q", got, plain)
	}
	if got := axAttrNames(editor); !slices.Contains(got, "AXSubrole") {
		t.Errorf("a search field's attributes %v", got)
	}
	v.native.Send(selRelease)
	delete(views, v.input.field)
	delete(views, v.input.host)

	status := &view{id: 1, kind: driver.KindView, native: objc.ID(classView).Send(selNew)}
	views[status.native] = status
	status.setProps(&driver.ViewProps{Role: driver.RoleStatus, LiveRegion: driver.LiveRegionPolite})
	if got := apple.GoString(status.native.Send(sel("accessibilityRole"))); got != "AXGroup" {
		t.Errorf("a status is %q, want AXGroup", got)
	}
	if got := apple.GoString(status.native.Send(sel("accessibilitySubrole"))); got != "AXApplicationStatus" {
		t.Errorf("a status's subrole %q, want AXApplicationStatus", got)
	}
	if status.axGrouped {
		t.Error("a status hides its children")
	}
	delete(views, status.native)
	status.native.Send(selRelease)
}
