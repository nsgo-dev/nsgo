package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestShowDisposesBranchBeforeItsBindingsRun(t *testing.T) {
	var items *Signal[[]string]
	var seen []string
	builds, cleanups := 0, 0
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b"}, nil)
		return View{Children: []Node{
			Show{When: func() bool { return len(items.Get()) > 0 }, Then: func(cx *Scope) Node {
				builds++
				cx.Cleanup(func() { cleanups++ })
				return Text{TestID: "first", ContentFunc: func() string {
					first := items.Get()[0]
					seen = append(seen, first)
					return first
				}}
			}},
			// A reader after the branch changes the source's subscriber order as each computation unlinks and subscribes again.
			Text{ContentFunc: func() string { items.Get(); return "sibling" }},
		}}
	})
	defer h.root.Dispose()
	h.commit()
	items.Set([]string{"b"})
	h.commit()
	if builds != 1 || cleanups != 0 || h.find("first").text.content != "b" {
		t.Fatalf("unchanged condition rebuilt the branch: builds %d, cleanups %d", builds, cleanups)
	}
	items.Set(nil)
	b := h.commit()
	if len(h.errs) != 0 {
		t.Fatalf("hidden branch ran against an empty list: %v", h.errs[0])
	}
	if cleanups != 1 || kinds(b)[driver.ItemDelete] != 1 || !slices.Equal(seen, []string{"a", "b"}) {
		t.Fatalf("after hiding: cleanups %d, reads %q, batch %v", cleanups, seen, kinds(b))
	}
	items.Set([]string{"c"})
	h.commit()
	if builds != 2 || cleanups != 1 || h.find("first").text.content != "c" {
		t.Fatalf("branch did not reopen: builds %d, cleanups %d", builds, cleanups)
	}
}

func TestDynamicReplacesBranchBeforeItsBindingsRun(t *testing.T) {
	var items *Signal[[]string]
	var seen []string
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b"}, nil)
		return View{Children: []Node{
			Dynamic(func(cx *Scope) Node {
				if len(items.Get()) == 0 {
					return nil
				}
				return Text{TestID: "first", ContentFunc: func() string {
					first := items.Get()[0]
					seen = append(seen, first)
					return first
				}}
			}),
			Text{ContentFunc: func() string { items.Get(); return "sibling" }},
		}}
	})
	defer h.root.Dispose()
	h.commit()
	for _, next := range [][]string{{"b"}, nil, {"c"}, {"d"}, nil} {
		items.Set(next)
		h.commit()
	}
	if len(h.errs) != 0 || !slices.Equal(seen, []string{"a", "b", "c", "d"}) {
		t.Fatalf("replaced branch ran: reads %q, errors %v", seen, h.errs)
	}
}

func TestCatchDisposesFailedBranchBeforeItsBindingsRun(t *testing.T) {
	var items *Signal[[]string]
	var failure any
	reads := 0
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a"}, nil)
		return Catch{
			Try: func(cx *Scope) Node {
				return View{TestID: "fail", OnPress: func(PressEvent) {
					items.Set(nil)
					panic("boom")
				}, Children: []Node{Text{ContentFunc: func() string {
					reads++
					return items.Get()[0]
				}}}}
			},
			Fallback: func(cx *Scope, err error, reset func()) Node {
				failure = err.(*PanicError).Value
				return Text{TestID: "fallback", Content: "failed"}
			},
		}
	})
	defer h.root.Dispose()
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventPress, ID: h.find("fail").id})
	h.commit()
	h.find("fallback")
	if reads != 1 || failure != "boom" || len(h.errs) != 0 {
		t.Fatalf("failed branch ran again: reads %d, caught %v, reported %v", reads, failure, h.errs)
	}
}
