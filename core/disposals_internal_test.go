//go:build nsgodev || nsgotest || nsgocheck

package core

import (
	"strconv"
	"testing"
)

// Disposed branches and rows leave no stale entries in devState's disposal maps.
func TestDisposalsStateForgetsDisposedScopes(t *testing.T) {
	var items *Signal[[]int]
	var shown *Signal[bool]
	h := devHost(t, func(cx *Scope) Node {
		items = NewSignalFunc[[]int](cx, nil, nil)
		shown = NewSignal(cx, false)
		return Group{
			Each(items.Get, func(i int) int { return i }, func(cx *Scope, item func() int, _ func() int) Node {
				cx.Cleanup(func() {})
				return Text{ContentFunc: func() string { return strconv.Itoa(item()) }}
			}),
			Show{When: shown.Get, Then: func(cx *Scope) Node {
				cx.Cleanup(func() {})
				return View{}
			}},
		}
	})
	h.commit()
	d := h.root.rt.dev
	baseCleanups, baseLabels := len(d.cleanups), len(d.scopeLabels)

	items.Set([]int{1, 2, 3})
	shown.Set(true)
	h.commit()
	items.Set(nil)
	shown.Set(false)
	h.commit()

	if len(d.cleanups) != baseCleanups || len(d.scopeLabels) != baseLabels {
		t.Fatalf("disposal maps leaked after disposal: cleanups=%d (want %d), scopeLabels=%d (want %d)",
			len(d.cleanups), baseCleanups, len(d.scopeLabels), baseLabels)
	}
}
