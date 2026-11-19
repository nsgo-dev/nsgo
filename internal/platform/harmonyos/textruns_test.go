//go:build js && wasm

package harmonyos

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestTextDrawsNoAttachmentRunes keeps the rune an inline view stands on out of what ArkUI draws, which would show it as a box, in a plain text and in a text's runs alike.
func TestTextDrawsNoAttachmentRunes(t *testing.T) {
	text := "an icon " + string(driver.AttachmentRune) + " inline"
	if got := drawnText(text); got != "an icon  inline" {
		t.Errorf("drawn text %q, want the rune left out", got)
	}
	runs := textRunsJSON(&driver.TextContent{Text: text,
		Spans: []driver.TextSpan{{Start: 0, End: 3}, {Start: 3, End: len(text), Style: driver.TextStyle{Color: 0xff0000ff}}}})
	if runs == "" || strings.ContainsRune(runs, driver.AttachmentRune) {
		t.Errorf("runs %s, want them without the attachment rune", runs)
	}
}
