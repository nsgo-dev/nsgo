//go:build linux && !android

package linux

import (
	"slices"
	"testing"
	"time"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

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
