package core

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestPeerClassesRequireSiblingContext(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)

	const class = "text-[#123456] peer-checked:text-[#ff0000] dark:peer-hover/field:!text-[#00ff00]"
	var refresh *Signal[int]
	var scope *Scope
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		scope = cx
		refresh = NewSignal(cx, 0)
		return View{Children: []Node{
			View{Class: "peer", Checked: func() bool { return true }},
			Text{TestID: "text", Class: class, SpansFunc: func() []Span {
				refresh.Get()
				return []Span{{Content: "x", Class: class}, {Content: "y", Class: class}}
			}},
			Stack{Children: []Node{Screen{TestID: "screen", HeaderClass: class}}},
		}}
	})
	for range 3 {
		refresh.Update(func(i int) int { return i + 1 })
		// A new theme also reparses the existing span and header lists.
		theme := *DefaultTheme()
		scope.SetTheme(&theme)
		h.commit()
	}
	for _, where := range []string{"span classes", "HeaderClass"} {
		for _, token := range []string{"peer-checked:text-[#ff0000]", "dark:peer-hover/field:!text-[#00ff00]"} {
			line := `class "` + token + `" is unsupported in ` + where
			if got := strings.Count(output.String(), line); got != 1 {
				t.Errorf("%q logged %d times, want once:\n%s", line, got, output.String())
			}
		}
	}
	if strings.Contains(output.String(), "unknown class") {
		t.Errorf("a supported view class was called unknown:\n%s", output.String())
	}
	n := h.find("text")
	if n.style.Color != RGB(0xff0000) {
		t.Errorf("the same cached classes no longer apply to a sibling view: %x", n.style.Color)
	}
	for _, span := range n.text.spans {
		if span.style.Color != RGB(0x123456) {
			t.Errorf("normal span classes were lost: %x", span.style.Color)
		}
	}
	if color := h.find("screen").screen.headerClass.Color; color != RGB(0x123456) {
		t.Errorf("normal header classes were lost: %x", color)
	}
}
