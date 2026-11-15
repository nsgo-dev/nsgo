//go:build darwin

package apple

import (
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestFontListTakesTheFirstFamilyInstalled(t *testing.T) {
	family := func(list string) string {
		font := Font(driver.Font{Family: list, Size: 12})
		defer Release(font)
		name := ctFontCopyFamilyName(font)
		defer Release(name)
		return GoString(objc.ID(name))
	}
	if got := family(`"No Such Family", Menlo, monospace`); got != "Menlo" {
		t.Errorf("the list set in %q, want Menlo", got)
	}
	if got := family("Menlo"); got != "Menlo" {
		t.Errorf("one family set in %q", got)
	}
}

func TestStyledFacesOfNamedFamilies(t *testing.T) {
	face := func(f driver.Font) string {
		font := Font(f)
		defer Release(font)
		name := ctFontCopyPostScriptName(font)
		defer Release(name)
		return GoString(objc.ID(name))
	}
	for _, family := range []string{driver.FontMonospace, "Menlo", `"No Such Family", Menlo`} {
		for want, f := range map[string]driver.Font{
			"Menlo-Bold":       {Family: family, Size: 12, Weight: 700},
			"Menlo-Italic":     {Family: family, Size: 12, Italic: true},
			"Menlo-BoldItalic": {Family: family, Size: 12, Weight: 700, Italic: true},
		} {
			if family == driver.FontMonospace && face(driver.Font{Family: family, Size: 12}) != "Menlo-Regular" {
				continue // the user's fixed-pitch font is not Menlo
			}
			if got := face(f); got != want {
				t.Errorf("%s: %+v sets in %s", want, f, got)
			}
		}
	}
	if got := face(driver.Font{Size: 12, Weight: 700}); !strings.Contains(got, "Bold") && !strings.Contains(got, "SFNS") {
		t.Errorf("the system's bold sets in %s", got)
	}
}

func TestStyledFacesOfFallbackFamilies(t *testing.T) {
	font := Font(driver.Font{Family: "Menlo, Times New Roman", Size: 12, Weight: 700, Italic: true})
	defer Release(font)
	desc := ctFontCopyFontDescriptor(font)
	defer Release(desc)
	cascade := ctFontDescriptorCopyAttr(desc, attrCascadeList)
	defer Release(cascade)
	if n := cfArrayGetCount(cascade); n != 1 {
		t.Fatalf("cascade has %d entries, want Times New Roman", n)
	}
	fallback := ctFontCreateWithDescriptor(cfArrayGetValueAtIndex(cascade, 0), 12, 0)
	defer Release(fallback)
	name := ctFontCopyPostScriptName(fallback)
	defer Release(name)
	if got := GoString(objc.ID(name)); !strings.Contains(got, "TimesNewRomanPS-BoldItalic") {
		t.Fatalf("styled fallback face is %s, want Times New Roman bold italic", got)
	}
}

// TestListsFallBackInTheStyledFace lays out characters the first family lacks and checks the face CoreText sets them in is the later family's at the weight and slant asked for, as that family alone would be.
func TestListsFallBackInTheStyledFace(t *testing.T) {
	for _, c := range []struct {
		list, fallback, text string
		weight               driver.FontWeight
		italic               bool
	}{
		{"Menlo, Hiragino Sans", "Hiragino Sans", "a中", 700, false},
		{"system-ui, Hiragino Sans", "Hiragino Sans", "a中", 700, false},
		{"Didot, Times New Roman", "Times New Roman", "aЖ", driver.FontWeightNormal, true},
	} {
		if f := familyFont(driver.Family{Name: c.fallback}, 12, ""); f == 0 {
			t.Logf("%s is not installed", c.fallback)
			continue
		} else {
			Release(f)
		}
		alone := Font(driver.Font{Family: c.fallback, Size: 12, Weight: c.weight, Italic: c.italic})
		name := ctFontCopyPostScriptName(alone)
		want := GoString(objc.ID(name))
		Release(name)
		Release(alone)
		st := driver.TextStyle{Font: driver.Font{Family: c.list, Size: 12, Weight: c.weight, Italic: c.italic}, Color: driver.RGB(0)}
		faces := runFaces(st, c.text)
		if len(faces) != 2 || faces[1] != want {
			t.Errorf("%q at %d, italic %v: runs set in %v, want the second in %s", c.list, c.weight, c.italic, faces, want)
		}
	}
}

// TestLanguagePicksHanForms sets the same Han character in Japanese and in Chinese and checks CoreText falls back to each language's face, whatever the system prefers, and that the font cache keeps the two apart.
func TestLanguagePicksHanForms(t *testing.T) {
	face := func(lang string) string {
		faces := runFaces(driver.TextStyle{Font: driver.Font{Size: 12, Weight: driver.FontWeightNormal, Language: lang}, Color: driver.RGB(0)}, "直")
		if len(faces) != 1 {
			t.Fatalf("%s: runs set in %v, want one", lang, faces)
		}
		return faces[0]
	}
	for range 2 { // the second round reads the cache
		if got := face("ja"); !strings.Contains(got, "Hira") {
			t.Errorf("Japanese sets in %s, want a Hiragino face", got)
		}
		if got := face("zh-Hans"); !strings.Contains(got, "PingFang") || !strings.Contains(got, "SC") {
			t.Errorf("Simplified Chinese sets in %s, want a PingFang SC face", got)
		}
	}
}

// runFaces returns the PostScript names of the faces text's runs are set in.
func runFaces(st driver.TextStyle, text string) []string {
	str := attributed(&driver.AttributedText{Text: text, Style: st}, nil, nil)
	defer Release(str)
	line := ctLineCreateWithAttrString(str)
	defer Release(line)
	var out []string
	runs := ctLineGetGlyphRuns(line)
	for i := range cfArrayGetCount(runs) {
		font := cfDictionaryGetValue(ctRunGetAttributes(cfArrayGetValueAtIndex(runs, i)), attrFont)
		name := ctFontCopyPostScriptName(font)
		out = append(out, GoString(objc.ID(name)))
		Release(name)
	}
	return out
}
