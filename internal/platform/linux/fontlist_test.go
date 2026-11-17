//go:build linux && !android

package linux

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestEditorsFollowTheUIFont reads each live editor's CSS after GTK's setting changes, without reconfiguring its input. Both GTK editor types must follow system families in a list while explicit families keep their CSS.
func TestEditorsFollowTheUIFont(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	families := []string{"system-ui", "sans-serif", "ui-sans-serif", "ui-rounded", "serif, system-ui", "serif", `"system-ui"`, ""}
	title := "nsgo-editor-ui-font"
	w := startTestWindow(t, func(cx *core.Scope) core.Node {
		cx.SetTitle(title + "|ready")
		var children []core.Node
		for _, multiline := range []bool{false, true} {
			for i, family := range families {
				children = append(children, core.Input{TestID: fmt.Sprintf("%t-%d", multiline, i), Multiline: multiline,
					Style: core.Style{Width: core.Pt(200), Height: core.Pt(24), FontFamily: family}})
			}
		}
		return core.View{Children: children}
	}, Options{Title: title, Width: 240, Height: 420})
	waitForTitle(t, w, title+"|ready")
	var setSetting func(object uintptr, name, value *byte, end uintptr)
	var providerString func(provider uintptr) uintptr
	purego.RegisterLibFunc(&setSetting, gobjectLib, "g_object_set")
	purego.RegisterLibFunc(&providerString, gtkLib, "gtk_css_provider_to_string")
	var before string
	w.onLoop(t, func() { before = settingString(gtkSettingsGetDefault(), "gtk-font-name") })
	t.Cleanup(func() {
		w.onLoop(t, func() { setSetting(gtkSettingsGetDefault(), cstr("gtk-font-name"), cstr(before), 0) })
	})
	styles := func() map[string]string {
		out := map[string]string{}
		w.onLoop(t, func() {
			for _, v := range w.entries {
				if v.provider != 0 {
					p := providerString(v.provider)
					out[v.props.TestID] = goString(p)
					gFree(p)
				}
			}
		})
		return out
	}
	original := styles()
	if len(original) != 2*len(families) {
		t.Fatalf("editor styles = %d, want %d", len(original), 2*len(families))
	}
	for _, next := range []string{"Monospace", "Serif"} {
		w.onLoop(t, func() { setSetting(gtkSettingsGetDefault(), cstr("gtk-font-name"), cstr(next+" 11"), 0) })
		for id, css := range styles() {
			var multiline bool
			var index int
			fmt.Sscanf(id, "%t-%d", &multiline, &index)
			if index < 5 {
				if !strings.Contains(css, next) {
					t.Errorf("%s (%q) kept old CSS after the UI font became %s:\n%s", id, families[index], next, css)
				}
			} else if css != original[id] {
				t.Errorf("%s (%q) changed its explicit/inherited CSS", id, families[index])
			}
		}
	}
}

// TestFontNameFamilyIsPangosParse reads gtk-font-name's family as GTK does: styles, weights and the size are not words of it.
func TestFontNameFamilyIsPangosParse(t *testing.T) {
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	for name, want := range map[string]string{
		"Cantarell 11":               "Cantarell",
		"Noto Sans CJK SC 12":        "Noto Sans CJK SC",
		"Cantarell Bold Italic 11":   "Cantarell",
		"Ubuntu Condensed 10.5":      "Ubuntu",
		"Inter, Noto Sans CJK JP 10": "Inter,Noto Sans CJK JP",
		"11":                         "",
		"":                           "",
	} {
		if got := fontNameFamily(name); got != want {
			t.Errorf("fontNameFamily(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestSystemFamiliesAreTheUIFont gives a text with no family, and one naming the system's families, the UI font, as Apple and Windows give them the system font, and leaves other generic families alone.
func TestSystemFamiliesAreTheUIFont(t *testing.T) {
	ui := "Cantarell,Noto Sans CJK SC"
	for family, want := range map[string][]string{
		"":                       {"Cantarell", "Noto Sans CJK SC"},
		"system-ui":              {"Cantarell", "Noto Sans CJK SC"},
		"sans-serif":             {"Cantarell", "Noto Sans CJK SC"},
		"Inter, system-ui":       {"Inter", "Cantarell", "Noto Sans CJK SC"},
		"serif":                  {"serif"},
		driver.FontMonospace:     {"monospace"},
		`"system-ui", monospace`: {"system-ui", "monospace"},
	} {
		if got := familyNames(family, ui); !slices.Equal(got, want) {
			t.Errorf("familyNames(%q) = %q, want %q", family, got, want)
		}
	}
}

// TestTextFollowsTheUIFont shows texts with no family and changes GTK's gtk-font-name to a monospaced font, as the desktop's settings do: the app measures them again in it, so a line of i and one of M come out as wide.
func TestTextFollowsTheUIFont(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	var gObjectSet func(object uintptr, name, value *byte, end uintptr)
	title := "nsgo-ui-font"
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title + "|ready")
		return core.View{Style: core.Style{AlignItems: core.AlignStart, Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.Text{Content: "iiiiiiii", Style: core.Style{FontSize: 20}},
			core.Text{Content: "MMMMMMMM", Style: core.Style{FontSize: 20}},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 320, Height: 200})
	waitForTitle(t, w, title+"|ready")
	purego.RegisterLibFunc(&gObjectSet, gobjectLib, "g_object_set")
	var before string
	w.onLoop(t, func() { before = settingString(gtkSettingsGetDefault(), "gtk-font-name") })
	t.Cleanup(func() {
		w.onLoop(t, func() { gObjectSet(gtkSettingsGetDefault(), cstr("gtk-font-name"), cstr(before), 0) })
	})
	widths := func() (narrow, wide driver.Points) {
		w.onLoop(t, func() {
			for _, v := range w.views {
				if v == nil || v.kind != driver.KindText {
					continue
				}
				switch v.plain {
				case "iiiiiiii":
					narrow = v.frame.Size.Width
				case "MMMMMMMM":
					wide = v.frame.Size.Width
				}
			}
		})
		return narrow, wide
	}
	if narrow, wide := widths(); narrow == 0 || narrow >= wide {
		t.Fatalf("in %q, i measured %v and M %v", before, narrow, wide)
	}
	w.onLoop(t, func() { gObjectSet(gtkSettingsGetDefault(), cstr("gtk-font-name"), cstr("Monospace 11"), 0) })
	if got := uiFontFamily(); got != "Monospace" {
		t.Errorf("the UI font is %q after gtk-font-name changed to Monospace 11", got)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		narrow, wide := widths()
		if narrow > 0 && narrow == wide {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("in a monospaced UI font, i measured %v and M %v", narrow, wide)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
