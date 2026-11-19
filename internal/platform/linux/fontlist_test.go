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
		after := styles()
		if len(after) != len(original) {
			t.Fatalf("editor styles = %d after the UI font became %s, want %d", len(after), next, len(original))
		}
		for id, css := range after {
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

// TestInputsDrawTheirTextAsTextDoes draws one string in a text, a field and a multiline input of the same style and no family, and compares the width and height of their ink, at the session's UI font and after gtk-font-name changes (#764). An input's font comes from GTK's default font, which follows gtk-font-name, where a text's comes from the UI font read from it.
func TestInputsDrawTheirTextAsTextDoes(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	const content = "Hxg text one"
	title := "nsgo-input-text-font"
	row := func(n core.Node) core.Node {
		return core.View{Style: core.Style{Height: core.Pt(40)}, Children: []core.Node{n}}
	}
	style := core.Style{FontSize: 16, Color: driver.RGB(0)}
	w := startTestWindow(t, func(cx *core.Scope) core.Node {
		cx.SetTitle(title + "|ready")
		return core.View{Style: core.Style{AlignItems: core.AlignStart, Background: driver.RGB(0xffffff)}, Children: []core.Node{
			row(core.Text{Content: content, Style: style}),
			row(core.Input{DefaultValue: content, HideCaret: true, Style: style.Merge(core.Style{Width: core.Pt(300), Height: core.Pt(30)})}),
			row(core.Input{Multiline: true, DefaultValue: content, HideCaret: true, Style: style.Merge(core.Style{Width: core.Pt(300), Height: core.Pt(40)})}),
		}}
	}, Options{Title: title, Width: 320, Height: 200})
	waitForTitle(t, w, title+"|ready")
	var gObjectSet func(object uintptr, name, value *byte, end uintptr)
	purego.RegisterLibFunc(&gObjectSet, gobjectLib, "g_object_set")
	var before string
	w.onLoop(t, func() { before = settingString(gtkSettingsGetDefault(), "gtk-font-name") })
	t.Cleanup(func() {
		w.onLoop(t, func() { gObjectSet(gtkSettingsGetDefault(), cstr("gtk-font-name"), cstr(before), 0) })
	})
	// ink is the size of what is drawn darker than mid-grey in the row from y.
	ink := func(p pixels, y int) (width, height int) {
		minX, maxX, minY, maxY := p.width, -1, y+40, -1
		for yy := y; yy < y+40 && yy < p.height; yy++ {
			for x := 0; x < p.width; x++ {
				if px := p.at(x, yy); int(px[0])+int(px[1])+int(px[2]) < 3*128 {
					minX, maxX, minY, maxY = min(minX, x), max(maxX, x), min(minY, yy), max(maxY, yy)
				}
			}
		}
		if maxX < 0 {
			return 0, 0
		}
		return maxX - minX + 1, maxY - minY + 1
	}
	// near allows a pixel either way: a line placed at another fraction of a pixel antialiases into a row or column more, where another font differs by several.
	near := func(a, b [2]int) bool {
		return a[0]-b[0] <= 1 && b[0]-a[0] <= 1 && a[1]-b[1] <= 1 && b[1]-a[1] <= 1
	}
	alike := func(font string) {
		t.Helper()
		var got [3][2]int
		deadline := time.Now().Add(5 * time.Second)
		for {
			p := w.surfacePixels(t)
			for i := range got {
				got[i][0], got[i][1] = ink(p, 40*i)
			}
			if got[0][0] > 0 && near(got[1], got[0]) && near(got[2], got[0]) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("in %s, the ink of the text, the field and the multiline input is %v wide and tall", font, got)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	alike(before)
	for _, next := range []string{"Monospace 11", "Serif 13"} {
		w.onLoop(t, func() { gObjectSet(gtkSettingsGetDefault(), cstr("gtk-font-name"), cstr(next), 0) })
		alike(next)
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

// TestOnlySystemFamiliesNameTheUIFont picks the editors whose CSS a UI font change reloads: those naming a system family anywhere in their list, not those with explicit or generic families, quoted names or none.
func TestOnlySystemFamiliesNameTheUIFont(t *testing.T) {
	for family, want := range map[string]bool{
		"system-ui":              true,
		"sans-serif":             true,
		"ui-rounded":             true,
		"Inter, system-ui":       true,
		"":                       false,
		"serif":                  false,
		"Inter":                  false,
		driver.FontMonospace:     false,
		`"system-ui", monospace`: false,
	} {
		if got := namesUIFont(family); got != want {
			t.Errorf("namesUIFont(%q) = %t, want %t", family, got, want)
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
