package core

import (
	"strings"
	"testing"
)

// An Em canvas measures in ems of its font, follows the font size and the user's text size, and gives way to a size its style sets.
func TestEmCanvasFollowsItsFont(t *testing.T) {
	var size *Signal[Points]
	var em *Signal[EmBox]
	h := newHost(t, Size{Width: 400, Height: 200}, func(cx *Scope) Node {
		size = NewSignal[Points](cx, 20)
		em = NewSignal(cx, EmBox{Width: 2, Ascent: 0.8, Descent: 0.2})
		return View{StyleFunc: func() Style { return Style{FontSize: size.Get(), AlignItems: AlignStart} }, Children: []Node{
			Canvas{TestID: "em", EmFunc: em.Get},
			Canvas{TestID: "sized", Em: EmBox{Width: 2, Ascent: 1}, Style: Style{Width: Pt(5), Height: Pt(7)}},
		}}
	})
	h.commit()
	want := func(id string, w, ht Points) {
		t.Helper()
		if got := h.find(id).frame.Size; got != (Size{Width: w, Height: ht}) {
			t.Errorf("%s: size %v, want %vx%v", id, got, w, ht)
		}
	}
	want("em", 40, 20)
	want("sized", 5, 7)
	size.Set(30)
	h.commit()
	want("em", 60, 30)
	h.tree.SetEnvironment(Environment{FontScale: 1.5})
	h.commit()
	want("em", 90, 45)
	em.Set(EmBox{Width: 1, Ascent: 1, Descent: 1})
	h.commit()
	want("em", 45, 90)
}

// In a text, an Em canvas sits on the baseline by its own, copies as its span's Content, reads as its label, and takes the text's limits on the user's text size.
func TestEmCanvasAttachment(t *testing.T) {
	var descent *Signal[float32]
	var label *Signal[string]
	h := newHost(t, Size{Width: 400, Height: 200}, func(cx *Scope) Node {
		descent = NewSignal[float32](cx, 0.25)
		label = NewSignal(cx, "x squared")
		formula := func(id string) Node {
			return Canvas{TestID: id, LabelFunc: label.Get, EmFunc: func() EmBox { return EmBox{Width: 1, Ascent: 1 - descent.Get(), Descent: descent.Get()} }}
		}
		return View{Style: Style{FontSize: 20}, Children: []Node{
			Text{TestID: "text", Spans: []Span{{Content: "a "}, {Content: "$x^2$", Attachment: formula("formula")}, {Content: " b"}}},
			Text{TestID: "fixed", NoFontScaling: true, Spans: []Span{{Attachment: formula("fixed formula")}}},
			Text{TestID: "capped", MaxFontSizeMultiplier: 1.5, Spans: []Span{{Attachment: formula("capped formula")}}},
			View{TestID: "group", Accessible: true, Children: []Node{
				Text{Spans: []Span{{Content: "a "}, {Content: "$x^2$", Attachment: formula("read")}, {Content: "hidden", Style: Style{Visibility: VisibilityHidden}}}},
			}},
		}}
	})
	h.commit()
	text := h.find("text")
	at := text.text.text
	if len(at.Attachments) != 1 || at.Attachments[0].Descent != 5 || at.Attachments[0].Size != (Size{Width: 20, Height: 20}) {
		t.Fatalf("attachments %+v, want a 20-point square reaching 5 below the baseline", at.Attachments)
	}
	if sp := at.Spans[1]; sp.Copy != "$x^2$" {
		t.Errorf("the attachment's span copies as %q", sp.Copy)
	}
	if got := h.find("formula").props.Label; got != "x squared" {
		t.Errorf("label %q", got)
	}
	read := h.find("read").parent.parent
	if got := read.sentProps().Label; got != "a x squared" {
		t.Errorf("a text with hidden spans reads %q, not its formula's TeX", got)
	}
	if got := h.find("group").sentProps().Label; strings.Contains(got, "$") || !strings.Contains(got, "x squared") {
		t.Errorf("a group around a formula reads %q", got)
	}
	frame := h.find("formula").parent.frame
	height := text.frame.Size.Height
	descent.Set(0.5)
	h.commit()
	if got := text.frame.Size.Height; got <= height {
		t.Errorf("text height %v after the formula reached further below the baseline, was %v: the text was measured from its old size", got, height)
	}
	if got := text.text.text.Attachments[0].Descent; got != 10 {
		t.Errorf("descent %v after Em.Descent changed: the text was not prepared again", got)
	}
	if moved := h.find("formula").parent.frame; moved.Origin.Y != frame.Origin.Y+5 {
		t.Errorf("frame %v, was %v: a deeper descent puts its bottom further below the baseline", moved, frame)
	}
	label.Set("x cubed")
	h.commit()
	if got := h.find("formula").props.Label; got != "x cubed" {
		t.Errorf("label %q after LabelFunc changed", got)
	}

	h.tree.SetEnvironment(Environment{FontScale: 2})
	h.commit()
	for id, want := range map[string]Points{"formula": 40, "fixed formula": 20, "capped formula": 30} {
		if got := h.find(id).frame.Size.Height; got != want {
			t.Errorf("%s: height %v at text size 2, want %v", id, got, want)
		}
	}
}
