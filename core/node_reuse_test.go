package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestUncreatedNodeCommandsDoNotReachReplacement(t *testing.T) {
	for _, scroll := range []bool{false, true} {
		name := "focus"
		if scroll {
			name = "scroll"
		}
		t.Run(name, func(t *testing.T) {
			var scope *Scope
			var parent *Element
			h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
				scope = cx
				parent = NewElement(cx, View{})
				return parent
			})
			made := map[NodeID]bool{}
			commandViews(t, made, h.commit())
			oldRef, nextRef := new(Ref), new(Ref)
			props := func(ref *Ref) Node {
				if scroll {
					return Scroll{Ref: ref, Style: Style{Height: Pt(60)}}
				}
				return Input{Ref: ref}
			}
			old := NewElement(scope, props(oldRef))
			parent.Insert(old, nil)
			if old.n.created {
				t.Fatal("the transient view must not have reached the backend")
			}
			if scroll {
				oldRef.ScrollTo(Point{Y: 25}, false)
				oldRef.ScrollTo(Point{Y: 50}, true)
			} else {
				oldRef.Focus()
			}
			old.Release()
			next := NewElement(scope, props(nextRef))
			parent.Insert(next, nil)
			b := h.commit()
			commandViews(t, made, b)
			if count := refCommands(b); count != 0 {
				t.Errorf("replacement received %d commands queued for the removed view", count)
			}
			if scroll {
				for _, n := range []*node{old.n, next.n} {
					if sc := n.scroll; sc.offset != (Point{}) || sc.issued != 0 || len(sc.pending) != 0 {
						t.Errorf("view %d kept an unsent scroll: offset %v, issued %d, pending %v", n.id, sc.offset, sc.issued, sc.pending)
					}
				}
				nextRef.ScrollTo(Point{Y: 10}, false)
			} else {
				nextRef.Focus()
			}
			b = h.commit()
			commandViews(t, made, b)
			if count := refCommands(b); count != 1 {
				t.Fatalf("replacement's own request sent %d commands, want 1", count)
			}
		})
	}
}

func TestUncreatedNodeIDsAreReusedAfterCommit(t *testing.T) {
	var scope *Scope
	var parent *Element
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		scope = cx
		parent = NewElement(cx, View{})
		return parent
	})
	slots := len(h.tree.nodes) + 1
	for i := range 100 {
		e := NewElement(scope, Input{})
		parent.Insert(e, nil)
		e.Release()
		b := h.commit()
		if command(b, driver.ItemDelete) != nil {
			t.Fatal("backend was asked to delete a view it never created")
		}
		if len(h.tree.nodes) != slots {
			t.Fatalf("after update %d: node table grew to %d slots, want %d", i, len(h.tree.nodes), slots)
		}
	}
}
