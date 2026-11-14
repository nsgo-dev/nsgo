package driver

import (
	"errors"
	"fmt"
	"testing"
)

func TestSVGMarkerLimits(t *testing.T) {
	for _, doc := range []string{
		`<svg viewBox="0 0 10 10"><marker id="m"><path d="M0 0L1 1" fill="none" marker-end="url(#m)"/></marker><path d="M0 0L5 5" stroke="blue" marker-end="url(#m)"/></svg>`,
		`<svg viewBox="0 0 10 10"><marker id="m"><path d="M0 0L1 1" stroke="red" marker-end="url(#m)"/></marker><path d="M0 0L5 5" stroke="blue" marker-end="url(#m)"/></svg>`,
		`<svg viewBox="0 0 10 10"><marker id="a"><path d="M0 0L1 1" stroke="red" marker-end="url(#b)"/></marker><marker id="b"><path d="M0 0L1 1" stroke="red" marker-end="url(#a)"/></marker><path d="M0 0L5 5" marker-end="url(#a)"/></svg>`,
	} {
		// A marker never draws inside itself.
		if ops := svgOps(t, doc); len(ops) > 100 {
			t.Fatalf("a marker drawn inside itself recorded %d operations", len(ops))
		}
	}
	// Seven markers, each with ten mid vertices marked by the next, ask for ten million elements.
	doc := `<svg viewBox="0 0 10 10">`
	for i := range 7 {
		doc += fmt.Sprintf(`<marker id="m%d" overflow="visible"><path fill="none" d="M0 0h1h1h1h1h1h1h1h1h1h1h1" marker-mid="url(#m%d)"/></marker>`, i, i+1)
	}
	doc += `<path fill="none" d="M0 0h1h1h1h1h1h1h1h1h1h1h1" marker-mid="url(#m0)"/></svg>`
	if _, err := ParseSVG([]byte(doc)); !errors.Is(err, errSVGTooComplex) {
		t.Fatalf("ParseSVG = %v, want %v", err, errSVGTooComplex)
	}
	if _, err := ParseSVG([]byte(`<svg viewBox="0 0 10 10"><marker id="m"><path d="M0 0 X"/></marker><path d="M0 0L5 5" marker-end="url(#m)"/></svg>`)); err == nil {
		t.Fatal("ParseSVG drew a marker with bad path data without an error")
	}
}
