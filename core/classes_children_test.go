package core

import (
	"slices"
	"testing"
)

func TestPositionalClassesAcrossRegions(t *testing.T) {
	var items *Signal[[]string]
	var prefix *Signal[bool]
	row := func(id string) Node {
		return View{TestID: id, Class: "h-1 first:ml-1 last:mr-2 only:pt-3 odd:bg-red-500 even:bg-blue-500"}
	}
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b", "c"}, nil)
		prefix = NewSignal(cx, false)
		return View{TestID: "parent", Children: []Node{
			Show{When: prefix.Get, Then: func(*Scope) Node { return Group{row("prefix")} }},
			Each(items.Get, func(id string) string { return id }, func(_ *Scope, item func() string, _ func() int) Node {
				return Group{row(item())}
			}),
		}}
	})
	check := func(ids ...string) {
		t.Helper()
		h.commit()
		if got := h.ids(h.find("parent")); !slices.Equal(got, ids) {
			t.Fatalf("children %v, want %v", got, ids)
		}
		for i, id := range ids {
			var left, right, top Points
			if i == 0 {
				left = 4
			}
			if i == len(ids)-1 {
				right = 8
			}
			if len(ids) == 1 {
				top = 12
			}
			color := tailwindPalette["red"][5]
			if i%2 != 0 {
				color = tailwindPalette["blue"][5]
			}
			if s := h.find(id).style; s.Margin.Left != left || s.Margin.Right != right ||
				s.Padding.Top != top || s.Background != color {
				t.Errorf("%s at %d: margin %v, padding %v, background %#x", id, i, s.Margin, s.Padding, s.Background)
			}
		}
	}
	check("a", "b", "c")
	b := h.find("b").id
	items.Set([]string{"c", "b", "a", "d"})
	check("c", "b", "a", "d")
	if h.find("b").id != b {
		t.Fatal("reordering replaced a kept row")
	}
	prefix.Set(true)
	check("prefix", "c", "b", "a", "d")
	items.Set([]string{"b"})
	check("prefix", "b")
	prefix.Set(false)
	check("b")
	items.Set(nil)
	check()
	items.Set([]string{"d", "a"})
	check("d", "a")
}

func TestPositionalClassesFollowMovedElements(t *testing.T) {
	var left, right, region, row, plain *Element
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		left = NewElement(cx, View{TestID: "left"})
		right = NewElement(cx, View{TestID: "right"})
		plain = NewElement(cx, View{Style: Style{Height: Pt(1)}})
		left.Insert(plain, nil)
		right.Insert(NewElement(cx, View{Style: Style{Height: Pt(1)}}), nil)
		region = NewElement(cx, nil)
		inner := NewElement(cx, nil)
		row = NewElement(cx, View{TestID: "row", Class: "h-1 first:ml-1 last:mr-2 only:pt-3 odd:bg-red-500 even:bg-blue-500"})
		inner.Insert(row, nil)
		region.Insert(inner, nil)
		return Group{left, right}
	})
	check := func(left, right, top Points, color Color) {
		t.Helper()
		h.commit()
		if s := h.find("row").style; s.Margin.Left != left || s.Margin.Right != right ||
			s.Padding.Top != top || s.Background != color {
			t.Fatalf("moved row: margin %v, padding %v, background %#x", s.Margin, s.Padding, s.Background)
		}
	}
	left.Insert(region, plain)
	check(4, 0, 0, tailwindPalette["red"][5])
	id := row.n.id
	right.Insert(region, nil)
	check(0, 8, 0, tailwindPalette["blue"][5])
	right.Remove(region)
	h.commit()
	// It returns alone, then gains a plain sibling, with the same native view.
	left.Remove(plain)
	left.Insert(region, nil)
	check(4, 8, 12, tailwindPalette["red"][5])
	left.Insert(plain, region)
	check(0, 8, 0, tailwindPalette["blue"][5])
	if row.n.id != id {
		t.Fatal("moving a region replaced its native child")
	}
}

func TestPositionalAndSpacingClassesCanBeRemoved(t *testing.T) {
	var classes *Signal[string]
	var sibling *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		classes = NewSignal(cx, "last:opacity-25 space-y-2")
		sibling = NewSignal(cx, false)
		return View{Children: []Node{
			View{TestID: "row", ClassFunc: classes.Get, Children: []Node{
				View{TestID: "first", Class: "h-1"}, View{Class: "h-1"},
			}},
			Show{When: sibling.Get, Then: func(*Scope) Node { return View{Class: "h-1"} }},
		}}
	})
	check := func(opacity Opacity, margin Points) {
		t.Helper()
		h.commit()
		if s := h.find("row").style; s.Opacity != opacity {
			t.Errorf("opacity %v, want %v", s.Opacity, opacity)
		}
		if s := h.find("first").style; s.Margin.Bottom != margin {
			t.Errorf("spacing %v, want %v", s.Margin.Bottom, margin)
		}
	}
	check(Alpha(.25), 8)
	sibling.Set(true)
	check(Opacity{}, 8)
	classes.Set("")
	check(Opacity{}, 0)
	sibling.Set(false)
	check(Opacity{}, 0)
	classes.Set("last:opacity-25 space-y-2")
	check(Alpha(.25), 8)
}

func TestPositionalSpacingFollowsReorderedRows(t *testing.T) {
	var items *Signal[[]string]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b"}, nil)
		return View{Children: []Node{
			Each(items.Get, func(id string) string { return id }, func(_ *Scope, item func() string, _ func() int) Node {
				return View{Class: "first:space-y-2", Children: []Node{
					View{TestID: item(), Class: "h-1"}, View{Class: "h-1"},
				}}
			}),
		}}
	})
	for _, ids := range [][]string{{"a", "b"}, {"b", "a"}, {"a"}, {"b", "a"}} {
		items.Set(ids)
		h.commit()
		for i, id := range ids {
			var margin Points
			if i == 0 {
				margin = 8
			}
			if got := h.find(id).style.Margin.Bottom; got != margin {
				t.Errorf("%s in %v: spacing %v, want %v", id, ids, got, margin)
			}
		}
	}
}
