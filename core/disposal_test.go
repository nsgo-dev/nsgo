package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

type disposalTextEngine struct{ texts []*disposalPreparedText }

func (e *disposalTextEngine) Prepare(at *driver.AttributedText) driver.PreparedText {
	p := &disposalPreparedText{PreparedText: ApproxTextEngine{}.Prepare(at)}
	e.texts = append(e.texts, p)
	return p
}

type disposalPreparedText struct {
	driver.PreparedText
	releases int
}

func (p *disposalPreparedText) Release() {
	p.releases++
	p.PreparedText.Release()
}

func TestDisposingATreeReleasesItsPreparedTexts(t *testing.T) {
	text := &disposalTextEngine{}
	root := testRoot(RootOptions{})
	tree := NewTree(root, text)
	tree.Resize(Size{Width: 200, Height: 200}, Geometry{})
	var value *Signal[string]
	tree.Mount(func(cx *Scope) Node {
		value = NewSignal(cx, "First text")
		return View{Children: []Node{
			Text{ContentFunc: value.Get},
			Input{Value: value.Get, Multiline: true, OnContentSizeChange: func(Size) {}},
		}}
	})
	tree.Commit()
	value.Set("Replacement text")
	tree.Commit()
	if len(text.texts) == 0 {
		t.Fatal("the tree prepared no text")
	}
	root.Dispose()
	root.Dispose()
	for i, p := range text.texts {
		if p.releases != 1 {
			t.Errorf("prepared text %d released %d times, want once", i, p.releases)
		}
	}
}
