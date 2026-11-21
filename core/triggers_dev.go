//go:build nsgodev || nsgotest || nsgocheck

package core

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

const (
	maxCommitTriggers = 64
	maxTriggerTargets = 32
)

// devSignalID keeps the DevTools signal ID on each Signal in dev and test builds
// without pinning the signal in devState's maps.
type devSignalID struct{ devID uint64 }

func (s *devSignalID) setDevID(id uint64)  { s.devID = id }
func (s *devSignalID) devSignalID() uint64 { return s.devID }

type devWrite struct {
	signalID   uint64
	signalSite string
	signalType string
	writeSite  string
	value      string
}

type pendingTrigger struct {
	write *devWrite
	via   string
}

type pendingList []pendingTrigger

type triggerKey struct {
	signalID  uint64
	writeSite string
}

type targetKey struct {
	kind, label, site, via string
}

type triggerRecord struct {
	write   devWrite
	targets map[targetKey]*driver.TriggerTarget
	order   []targetKey
}

// devTriggers holds the signal-write to computation/region trigger causal chain on devState while devtools.Enabled.
type devTriggers struct {
	activeWrite  *devWrite
	propagating  []pendingTrigger
	pending      map[*computation][]pendingTrigger
	compLabels   map[*computation]string
	triggers     map[triggerKey]*triggerRecord
	triggerOrder []triggerKey
}

func (d *devState) initTriggers() {
	d.pending = map[*computation][]pendingTrigger{}
	d.compLabels = map[*computation]string{}
	d.triggers = map[triggerKey]*triggerRecord{}
}

func (d *devState) forgetTriggers(s *Scope) {
	if s.comp != nil {
		delete(d.pending, s.comp)
		delete(d.compLabels, s.comp)
	}
}

func (d *devState) compLabel(c *computation) string {
	return d.compLabels[c]
}

// labelComp marks c as a named region computation (Show, Each, Dynamic, Catch, List, Presence, Suspense)
// and refines its site from fn when fn belongs to app code.
func (rt *runtime) labelComp(c *computation, label string, fn any) {
	if rt.dev == nil || c == nil {
		return
	}
	rt.dev.compLabels[c] = label
	if fn != nil {
		if dc := rt.dev.comps[c]; dc != nil && dc.site == "" {
			if site := funcSite(fn); isPath(site) {
				dc.site = site
			}
		}
	}
}

func noteSignalWriteStart[T any](s *Signal[T], v T) *devWrite {
	d := s.rt.dev
	if d == nil {
		return nil
	}
	prev := d.activeWrite
	w := &devWrite{
		signalID:  s.devID,
		writeSite: callerSite(3),
		value:     clip(fmt.Sprint(any(v))),
	}
	if dv := d.byID[s.devID]; dv != nil {
		w.signalSite = dv.site
		w.signalType = dv.typ
	}
	d.activeWrite = w
	return prev
}

func noteSignalWriteEnd(rt *runtime, prev *devWrite) {
	if rt.dev != nil {
		rt.dev.activeWrite = prev
	}
}

func (c *computation) noteMarkedDirty() {
	d := c.rt.dev
	if d == nil {
		return
	}
	if len(d.propagating) > 0 {
		for _, pt := range d.propagating {
			d.addPending(c, pt)
		}
		return
	}
	if d.activeWrite != nil {
		d.addPending(c, pendingTrigger{write: d.activeWrite})
	}
}

func (d *devState) addPending(c *computation, pt pendingTrigger) {
	list := d.pending[c]
	for _, existing := range list {
		if existing.write == pt.write && existing.via == pt.via {
			return
		}
	}
	if len(list) < maxTriggerTargets {
		d.pending[c] = append(list, pt)
	}
}

func (c *computation) beginRunCausal() pendingList {
	d := c.rt.dev
	if d == nil {
		return nil
	}
	pts := d.pending[c]
	delete(d.pending, c)
	return pts
}

func (c *computation) endRunCausal(pts pendingList, took time.Duration) {
	d := c.rt.dev
	if d == nil || len(pts) == 0 {
		return
	}
	dc := d.comps[c]
	// Internal memos (such as Show.When's) forward triggers through beginMemoPropagate without appearing as separate targets.
	if c.kind == memoKind && (dc == nil || !isPath(dc.site)) {
		return
	}
	site := ""
	if dc != nil {
		site = d.siteOf(c, dc)
	}
	kind := compKindName(c.kind)
	label := d.compLabels[c]
	for _, pt := range pts {
		if pt.write == nil {
			continue
		}
		tk := triggerKey{signalID: pt.write.signalID, writeSite: pt.write.writeSite}
		rec := d.triggers[tk]
		if rec == nil {
			if len(d.triggerOrder) >= maxCommitTriggers {
				continue
			}
			rec = &triggerRecord{write: *pt.write, targets: map[targetKey]*driver.TriggerTarget{}}
			d.triggers[tk] = rec
			d.triggerOrder = append(d.triggerOrder, tk)
		} else if pt.write.value != "" {
			rec.write.value = pt.write.value
		}
		k := targetKey{kind: kind, label: label, site: site, via: pt.via}
		tg := rec.targets[k]
		if tg == nil {
			if len(rec.order) >= maxTriggerTargets {
				continue
			}
			tg = &driver.TriggerTarget{Kind: kind, Label: label, Site: site, Via: pt.via}
			rec.targets[k] = tg
			rec.order = append(rec.order, k)
		}
		tg.Runs++
		tg.Took += took
	}
}

func (c *computation) beginMemoPropagate(pts pendingList) {
	d := c.rt.dev
	if d == nil || len(pts) == 0 {
		return
	}
	memoSite := ""
	if dc := d.comps[c]; dc != nil {
		// Only record app-authored memo sites as a "via" hop; internal memos (such as Show.When's) stay transparent.
		if isPath(dc.site) {
			memoSite = dc.site
		}
	}
	fwd := make([]pendingTrigger, 0, len(pts))
	for _, pt := range pts {
		via := pt.via
		if memoSite != "" {
			via = memoSite
		}
		fwd = append(fwd, pendingTrigger{write: pt.write, via: via})
	}
	d.propagating = fwd
}

func (c *computation) endMemoPropagate() {
	if c.rt.dev != nil {
		c.rt.dev.propagating = nil
	}
}

func (d *devState) takeTriggers(limit int) []driver.Trigger {
	if len(d.triggerOrder) == 0 {
		return nil
	}
	out := make([]driver.Trigger, 0, len(d.triggerOrder))
	for _, tk := range d.triggerOrder {
		rec := d.triggers[tk]
		targets := make([]driver.TriggerTarget, 0, len(rec.order))
		for _, k := range rec.order {
			targets = append(targets, *rec.targets[k])
		}
		slices.SortFunc(targets, func(a, b driver.TriggerTarget) int {
			if a.Took != b.Took {
				return cmp.Compare(b.Took, a.Took)
			}
			if a.Label != b.Label {
				return strings.Compare(a.Label, b.Label)
			}
			return strings.Compare(a.Site, b.Site)
		})
		out = append(out, driver.Trigger{
			SignalID:   rec.write.signalID,
			SignalSite: rec.write.signalSite,
			SignalType: rec.write.signalType,
			WriteSite:  rec.write.writeSite,
			Value:      rec.write.value,
			Targets:    targets,
		})
	}
	clear(d.triggers)
	d.triggerOrder = d.triggerOrder[:0]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
