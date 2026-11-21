//go:build nsgodev || nsgotest || nsgocheck

package core

import (
	"strconv"
	"testing"
)

// Disposed branches and rows leave no stale entries in devState's trigger maps.
func TestTriggersStateForgetsDisposedScopes(t *testing.T) {
	var items *Signal[[]int]
	var shown *Signal[bool]
	h := devHost(t, func(cx *Scope) Node {
		items = NewSignalFunc[[]int](cx, nil, nil)
		shown = NewSignal(cx, false)
		return Group{
			Each(items.Get, func(i int) int { return i }, func(cx *Scope, item func() int, _ func() int) Node {
				return Text{ContentFunc: func() string { return strconv.Itoa(item()) }}
			}),
			Show{When: shown.Get, Then: func(cx *Scope) Node {
				return View{}
			}},
		}
	})
	h.commit()
	d := h.root.rt.dev
	baseCompLabels, basePending := len(d.compLabels), len(d.pending)

	items.Set([]int{1, 2, 3})
	shown.Set(true)
	h.commit()
	items.Set(nil)
	shown.Set(false)
	h.commit()

	if len(d.compLabels) != baseCompLabels || len(d.pending) != basePending {
		t.Fatalf("trigger maps leaked after disposal: compLabels=%d (want %d), pending=%d (want %d)",
			len(d.compLabels), baseCompLabels, len(d.pending), basePending)
	}
}
