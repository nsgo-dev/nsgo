package core

import (
	"fmt"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// commandViews checks items in delivery order, including across commits.
func commandViews(t *testing.T, made map[NodeID]bool, b *driver.MountBatch) {
	t.Helper()
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemCreate:
			made[it.ID] = true
		case driver.ItemDelete:
			delete(made, it.ID)
		default:
			if it.ID != 0 && !made[it.ID] {
				t.Fatalf("item kind %d names view %d before its creation", it.Kind, it.ID)
			}
		}
	}
}

func refCommands(b *driver.MountBatch) int {
	n := 0
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemFocus, driver.ItemBlur, driver.ItemAccessibilityFocus, driver.ItemScrollTo, driver.ItemScrollBy, driver.ItemZoomTo:
			n++
		}
	}
	return n
}

func TestRefCommandsInUnplacedHolder(t *testing.T) {
	for _, pacing := range []bool{false, true} {
		for _, existed := range []bool{false, true} {
			t.Run(fmt.Sprintf("pacing=%v/created=%v", pacing, existed), func(t *testing.T) {
				var shown, content *Signal[bool]
				input, scroll, list := &Ref{}, &Ref{}, &Ref{}
				h := newHost(t, Size{Width: 200, Height: 300}, nil)
				h.tree.SetViewPacing(pacing)
				h.tree.Mount(func(cx *Scope) Node {
					shown, content = NewSignal(cx, true), NewSignal(cx, existed)
					held := NewHolder(cx, func(cx *Scope) Node {
						return Show{When: content.Get, Then: func(cx *Scope) Node {
							return View{Children: []Node{
								Input{Ref: input},
								Scroll{Ref: scroll, Style: Style{Height: Pt(50)}, Children: []Node{View{Style: Style{Height: Pt(500)}}}},
								List{Ref: list, Style: Style{Height: Pt(50)}, Count: func() int { return 20 }, EstimatedSize: 30,
									Row: func(*Scope, ListRow) Node { return View{Style: Style{Height: Pt(30)}} }},
							}}
						}}
					})
					return Show{When: shown.Get, Then: func(*Scope) Node { return held.Place() }}
				})
				made := map[NodeID]bool{}
				commit := func() *driver.MountBatch {
					b := h.commit()
					commandViews(t, made, b)
					return b
				}
				commit()
				shown.Set(false)
				commit()
				content.Set(true)
				commit()
				commands := func() {
					input.Focus()
					input.Blur()
					input.SetAccessibilityFocus()
					input.Reveal(RevealOptions{})
					input.RevealBox(Bounds{Size: Size{Width: 10, Height: 10}}, RevealOptions{})
					scroll.ScrollTo(Point{Y: 10}, false)
					scroll.ScrollToEnd(false)
					scroll.ZoomTo(2, nil, false)
					list.ScrollTo(Point{Y: 10}, false)
					list.ScrollToEnd(false)
					list.ScrollToIndex(3, false)
				}
				commands()
				if n := refCommands(commit()); n != 0 {
					t.Fatalf("unplaced content sent %d commands", n)
				}
				// Calls while detached are dropped even if it is placed before commit.
				commands()
				shown.Set(true)
				if n := refCommands(commit()); n != 0 {
					t.Fatalf("placing content replayed %d commands", n)
				}
				input.Focus()
				scroll.ScrollTo(Point{Y: 20}, false)
				if n := refCommands(commit()); n != 2 {
					t.Fatalf("placed content sent %d commands, want 2", n)
				}
				// A command queued while placed must not survive its removal.
				sc := scroll.n.scroll
				issued, pending, offset := sc.issued, len(sc.pending), sc.offset
				input.Focus()
				input.Blur()
				scroll.ScrollToEnd(false)
				list.ScrollToIndex(4, false)
				shown.Set(false)
				if n := refCommands(commit()); n != 0 {
					t.Fatalf("removal sent %d queued commands", n)
				}
				if sc.issued != issued || len(sc.pending) != pending || sc.offset != offset {
					t.Fatalf("dropped scroll changed count, pending or offset: %d, %d, %v", sc.issued, len(sc.pending), sc.offset)
				}
				shown.Set(true)
				if n := refCommands(commit()); n != 0 {
					t.Fatalf("placing content replayed %d queued commands", n)
				}
			})
		}
	}
}

func TestRefCommandsForDeletedViews(t *testing.T) {
	for _, pacing := range []bool{false, true} {
		t.Run(fmt.Sprint(pacing), func(t *testing.T) {
			var shown *Signal[bool]
			input, scroll := &Ref{}, &Ref{}
			h := newHost(t, Size{Width: 200, Height: 300}, nil)
			h.tree.SetViewPacing(pacing)
			h.tree.Mount(func(cx *Scope) Node {
				shown = NewSignal(cx, false)
				return Show{When: shown.Get, Then: func(cx *Scope) Node {
					cx.Effect(func() { scroll.ScrollToEnd(false) })
					return Scroll{Ref: scroll, Children: []Node{Input{Ref: input}}}
				}}
			})
			made := map[NodeID]bool{}
			commandViews(t, made, h.commit())
			shown.Set(true)
			input.Focus()
			input.Blur()
			shown.Set(false)
			b := h.commit()
			commandViews(t, made, b)
			if n := refCommands(b); n != 0 {
				t.Fatalf("deleted views sent %d commands", n)
			}
		})
	}
}

func TestRefCommandCreatesPacedView(t *testing.T) {
	ref := &Ref{}
	h := newHost(t, Size{Width: 200, Height: 50}, func(*Scope) Node {
		kids := make([]Node, 200)
		for i := range kids {
			kids[i] = View{Style: Style{Height: Pt(30)}}
		}
		kids = append(kids, Input{Ref: ref})
		return Scroll{Style: Style{Height: Pt(50)}, Children: kids}
	})
	made := map[NodeID]bool{}
	commandViews(t, made, h.commit())
	if ref.n.created {
		t.Fatal("test target did not wait for pacing")
	}
	ref.Focus()
	b := h.commit()
	commandViews(t, made, b)
	if !ref.n.created || refCommands(b) != 1 {
		t.Fatal("command did not create and focus the paced view")
	}
}

func TestUnplacedListKeepsItsInitialPosition(t *testing.T) {
	var shown, content *Signal[bool]
	ref := &Ref{}
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		shown, content = NewSignal(cx, true), NewSignal(cx, false)
		held := NewHolder(cx, func(cx *Scope) Node {
			return Show{When: content.Get, Then: func(*Scope) Node {
				return List{Ref: ref, ContentOffset: Point{Y: 120}, Style: Style{Height: Pt(60)},
					Count: func() int { return 30 }, EstimatedSize: 30,
					Row: func(*Scope, ListRow) Node { return View{Style: Style{Height: Pt(30)}} }}
			}}
		})
		return Show{When: shown.Get, Then: func(*Scope) Node { return held.Place() }}
	})
	h.commit()
	shown.Set(false)
	h.commit()
	content.Set(true)
	h.commit()
	ref.ScrollToIndex(20, false)
	h.commit()
	shown.Set(true)
	b := h.commit()
	var offsets []Point
	for _, it := range b.Items {
		if it.Kind == driver.ItemScrollTo && it.ID == ref.n.id {
			offsets = append(offsets, it.Point)
		}
	}
	if len(offsets) != 1 || offsets[0].Y != 120 {
		t.Fatalf("initial position = %v, want 120 without replaying the Ref command", offsets)
	}
}

func TestDroppedScrollKeepsLatestPlatformReport(t *testing.T) {
	var shown *Signal[bool]
	ref := &Ref{}
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		shown = NewSignal(cx, true)
		held := NewHolder(cx, func(*Scope) Node {
			return Scroll{Ref: ref, Style: Style{Height: Pt(60)}, Children: []Node{View{Style: Style{Height: Pt(500)}}}}
		})
		return Show{When: shown.Get, Then: func(*Scope) Node { return held.Place() }}
	})
	h.commit()
	ref.ScrollTo(Point{Y: 900}, false)
	h.commit()
	ref.ScrollTo(Point{Y: 50}, false)
	// The preceding scroll was clamped before the queued one is dropped.
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: ref.n.id, Point: Point{Y: 440}, Count: 1})
	shown.Set(false)
	h.commit()
	sc := ref.n.scroll
	if sc.offset.Y != 440 || sc.issued != 1 || len(sc.pending) != 0 {
		t.Fatalf("cancelled scroll: offset %v, count %d, pending %v", sc.offset, sc.issued, sc.pending)
	}
}

func TestUnplacedHolderKeepsCommandsForRetainedWebView(t *testing.T) {
	var shown *Signal[bool]
	web := &Ref{}
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		shown = NewSignal(cx, true)
		held := NewHolder(cx, func(*Scope) Node { return WebView{Ref: web, Source: WebHTML("<p>kept</p>", "")} })
		return Show{When: shown.Get, Then: func(*Scope) Node { return held.Place() }}
	})
	made := map[NodeID]bool{}
	commandViews(t, made, h.commit())
	shown.Set(false)
	commandViews(t, made, h.commit())
	web.PostMessage("background")
	b := h.commit()
	commandViews(t, made, b)
	if kinds(b)[driver.ItemWebCommand] != 1 {
		t.Fatal("retained web view lost its background command")
	}
}
