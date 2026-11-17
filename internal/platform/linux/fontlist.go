//go:build linux && !android

package linux

import (
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// defaultUIFont is the UI font before GTK's settings are read, or where they name none: GTK's own default family.
const defaultUIFont = "Sans"

// uiFont is the family of GTK's gtk-font-name, the desktop's UI font a GtkLabel shows, which a text with no family or a system family (system-ui, sans-serif) takes. The main thread reads it from GtkSettings (readUIFont), the engine thread prepares text in it.
var uiFont atomic.Pointer[string]

// uiFontFamily returns the UI font's family, a Pango family list.
func uiFontFamily() string {
	if f := uiFont.Load(); f != nil {
		return *f
	}
	return defaultUIFont
}

// readUIFont reads the UI font from GTK's settings, on the main thread, and reports whether it changed.
func readUIFont() bool {
	family := ""
	if settings := gtkSettingsGetDefault(); settings != 0 {
		family = fontNameFamily(settingString(settings, "gtk-font-name"))
	}
	if family == "" {
		family = defaultUIFont
	}
	old := uiFont.Swap(&family)
	return old == nil || *old != family
}

// fontNameFamily returns the family of a Pango font name such as gtk-font-name's "Cantarell Bold 11": "Cantarell". Pango parses it, as GTK does, so styles, weights and sizes are not taken for words of the family.
func fontNameFamily(name string) string {
	desc := pangoFontDescriptionFromString(cstr(name))
	defer pangoFontDescriptionFree(desc)
	return goString(pangoFontDescriptionGetFamily(desc))
}

// familyNames maps a fontFamily list to fontconfig's names. An empty list and the system's families are the UI font ui.
func familyNames(family, ui string) []string {
	families := driver.FontFamilies(family)
	if len(families) == 0 {
		families = []driver.Family{{Kind: driver.FamilySystem}}
	}
	var names []string
	for _, f := range families {
		switch f.Kind {
		case driver.FamilySerif:
			names = append(names, "serif")
		case driver.FamilyMonospace:
			names = append(names, "monospace")
		case driver.FamilySystem, driver.FamilyRounded:
			for n := range strings.SplitSeq(ui, ",") {
				if n = strings.TrimSpace(n); n != "" {
					names = append(names, n)
				}
			}
		default:
			names = append(names, f.Name)
		}
	}
	return names
}

// pangoFamilies is a fontFamily list as Pango takes one: separated by
// commas, so a name with a comma in it cannot be one of its families.
func pangoFamilies(family, ui string) string {
	names := familyNames(family, ui)
	kept := names[:0]
	for _, n := range names {
		if !strings.Contains(n, ",") {
			kept = append(kept, n)
		}
	}
	return strings.Join(kept, ",")
}

// gtkFamilies is a fontFamily list as GTK's CSS takes one.
func gtkFamilies(family string) string {
	names := familyNames(family, uiFontFamily())
	for i, n := range names {
		names[i] = strconv.Quote(n)
	}
	return strings.Join(names, ", ")
}
