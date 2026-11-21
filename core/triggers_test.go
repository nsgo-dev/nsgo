//go:build nsgodev || nsgotest || nsgocheck

package core_test

import (
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestCausalChainSignalToComputationsAndRegions(t *testing.T) {
	driver.TraceCauses, devtools.Enabled = true, true
	defer func() {
		driver.TraceCauses, devtools.Enabled = false, false
		for _, s := range devtools.Surfaces() {
			devtools.Unregister(s)
		}
	}()

	root := core.NewRoot(core.RootOptions{CheckCommits: true, Post: func(func()) {}, Clock: core.NewManualClock(time.Unix(0, 0))})
	defer root.Dispose()
	tree := core.NewTree(root, core.ApproxTextEngine{})
	tree.Resize(core.Size{Width: 200, Height: 200}, core.Geometry{})

	var (
		count, other *core.Signal[int]
		items        *core.Signal[[]string]
		mode         *core.Signal[string]
		countSite    string
		memoSite     string
		effectSite   string
		showSite     string
		eachSite     string
		dynSite      string
		listSite     string
		effectRuns   int
	)
	tree.Mount(func(cx *core.Scope) core.Node {
		count, countSite = core.NewSignal(cx, 0), line()
		other = core.NewSignal(cx, 0)
		items = core.NewSignalFunc[[]string](cx, []string{"a"}, nil)
		mode = core.NewSignal(cx, "one")

		positive := core.NewMemo(cx, func() bool { return count.Get() > 0 })
		memoSite = prevLine(1)

		cx.Effect(func() {
			_ = count.Get() + other.Get()
			effectRuns++
		})
		effectSite = prevLine(4)

		showThen := func(*core.Scope) core.Node { return core.Text{Content: "positive"} }
		showSite = prevLine(1)
		eachRow := func(_ *core.Scope, item func() string, _ func() int) core.Node {
			return core.Text{ContentFunc: item}
		}
		eachSite = prevLine(3)
		dynFn := func(*core.Scope) core.Node { return core.Text{Content: mode.Get()} }
		dynSite = prevLine(1)
		listRow := func(_ *core.Scope, r core.ListRow) core.Node {
			return core.Text{ContentFunc: func() string { return strconv.Itoa(r.Index()) }}
		}
		listSite = prevLine(3)

		return core.View{Children: []core.Node{
			core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }},
			core.Show{When: positive.Get, Then: showThen},
			core.Each(items.Get, func(s string) string { return s }, eachRow),
			core.Dynamic(dynFn),
			core.List{
				Count:         func() int { return len(items.Get()) },
				EstimatedSize: 20,
				Style:         core.Style{Height: core.Pt(60)},
				Row:           listRow,
			},
		}}
	})
	tree.Commit()

	// 1. Writing count (0 -> 1) triggers direct Text binding, Effect, Memo, AND indirectly Show via Memo.
	count.Set(1)
	writeCount1 := prevLine(1)
	b := tree.Commit()
	if b == nil || b.Causes == nil {
		t.Fatal("expected commit causes")
	}
	var countTrigger *driver.Trigger
	for i := range b.Causes.Triggers {
		if b.Causes.Triggers[i].WriteSite == writeCount1 {
			countTrigger = &b.Causes.Triggers[i]
			break
		}
	}
	if countTrigger == nil {
		t.Fatalf("no trigger for count.Set(1) at %s in %+v", writeCount1, b.Causes.Triggers)
	}
	if countTrigger.SignalID == 0 || countTrigger.SignalSite != countSite || countTrigger.SignalType != "int" || countTrigger.Value != "1" {
		t.Fatalf("trigger metadata = %+v, want site %s", *countTrigger, countSite)
	}
	hasTarget := func(tr *driver.Trigger, pred func(driver.TriggerTarget) bool) bool {
		return slices.ContainsFunc(tr.Targets, pred)
	}
	if !hasTarget(countTrigger, func(tg driver.TriggerTarget) bool { return tg.Kind == "binding" && tg.Label == "" }) {
		t.Errorf("missing direct binding target in %+v", countTrigger.Targets)
	}
	if !hasTarget(countTrigger, func(tg driver.TriggerTarget) bool { return tg.Kind == "effect" && tg.Site == effectSite }) {
		t.Errorf("missing effect target (%s) in %+v", effectSite, countTrigger.Targets)
	}
	if !hasTarget(countTrigger, func(tg driver.TriggerTarget) bool { return tg.Kind == "memo" && tg.Site == memoSite }) {
		t.Errorf("missing memo target (%s) in %+v", memoSite, countTrigger.Targets)
	}
	if !hasTarget(countTrigger, func(tg driver.TriggerTarget) bool {
		return tg.Label == "Show" && tg.Site == showSite && tg.Via == memoSite
	}) {
		t.Errorf("missing indirect Show target (site=%s via=%s) in %+v", showSite, memoSite, countTrigger.Targets)
	}

	// 2. Writing count (1 -> 2): positive memo stays true, so Show must NOT appear as triggered.
	count.Set(2)
	writeCount2 := prevLine(1)
	b = tree.Commit()
	countTrigger = nil
	for i := range b.Causes.Triggers {
		if b.Causes.Triggers[i].WriteSite == writeCount2 {
			countTrigger = &b.Causes.Triggers[i]
			break
		}
	}
	if countTrigger == nil {
		t.Fatalf("no trigger for count.Set(2) in %+v", b.Causes.Triggers)
	}
	if hasTarget(countTrigger, func(tg driver.TriggerTarget) bool { return tg.Label == "Show" }) {
		t.Errorf("unchanged memo falsely triggered Show: %+v", countTrigger.Targets)
	}

	// 3. Writing items triggers Each and List; writing mode triggers Dynamic.
	root.Batch(func() {
		items.Set([]string{"a", "b"})
		mode.Set("two")
	})
	b = tree.Commit()
	var sawEach, sawList, sawDynamic bool
	for _, tr := range b.Causes.Triggers {
		for _, tg := range tr.Targets {
			if tg.Label == "Each" && tg.Site == eachSite {
				sawEach = true
			}
			if tg.Label == "List" && tg.Site == listSite {
				sawList = true
			}
			if tg.Label == "Dynamic" && tg.Site == dynSite {
				sawDynamic = true
			}
		}
	}
	if !sawEach || !sawList || !sawDynamic {
		t.Fatalf("region triggers missing: Each=%v List=%v Dynamic=%v in %+v", sawEach, sawList, sawDynamic, b.Causes.Triggers)
	}

	// 4. Batched writes to both count and other attribute BOTH triggers to the shared effect.
	root.Batch(func() {
		count.Set(3)
		other.Set(10)
	})
	b = tree.Commit()
	effectTriggers := 0
	for _, tr := range b.Causes.Triggers {
		if hasTarget(&tr, func(tg driver.TriggerTarget) bool { return tg.Kind == "effect" && tg.Site == effectSite }) {
			effectTriggers++
		}
	}
	if effectTriggers != 2 {
		t.Fatalf("batched writes attributed effect to %d signals, want 2: %+v", effectTriggers, b.Causes.Triggers)
	}
	_ = effectRuns
}

// prevLine returns file:(callerLine - delta), matching DevTools' full pathSite format.
func prevLine(delta int) string {
	_, file, n, _ := runtime.Caller(1)
	return file + ":" + strconv.Itoa(n-delta)
}
