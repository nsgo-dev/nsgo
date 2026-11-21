//go:build nsgodev || nsgotest || nsgocheck

package core

import (
	goruntime "runtime"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

const (
	maxCommitDisposal = 64
	maxDisposalLog    = 100
)

type devScopeLabel struct {
	name string
	site string
}

type devCleanup struct {
	idx       int
	kind      string
	detail    string
	site      string
	handle    any
	ownerName string
	ownerSite string
	closed    bool
}

type disposalFrame struct {
	root      *Scope
	owner     string
	ownerSite string
	reason    string
	site      string
	cleanups  []driver.CleanupEvent
	prev      *disposalFrame
}

type disposalGuard struct {
	d     *devState
	frame *disposalFrame
}

// devDisposals holds active resource ownership and owner-disposal cleanup causal chains on devState while devtools.Enabled.
type devDisposals struct {
	scopeLabels     map[*Scope]devScopeLabel
	cleanups        map[*Scope][]devCleanup
	disposing       *disposalFrame
	commitDisposals []driver.Disposal
	disposalHistory []driver.Disposal
}

func (d *devState) initDisposals() {
	d.scopeLabels = map[*Scope]devScopeLabel{}
	d.cleanups = map[*Scope][]devCleanup{}
}

func (d *devState) forgetDisposals(s *Scope) {
	delete(d.scopeLabels, s)
	delete(d.cleanups, s)
}

func (d *devState) scopeLabelOf(o *Scope) (string, string) {
	if lbl, ok := d.scopeLabels[o]; ok {
		return lbl.name, lbl.site
	}
	return "", ""
}

// labelScope gives s a human-readable owner name and creation site (such as a window, panel, overlay or holder).
func (rt *runtime) labelScope(s *Scope, name, detail string) {
	if rt.dev == nil || s == nil {
		return
	}
	full := name
	if detail != "" {
		full = name + " " + strconv.Quote(detail)
	}
	rt.dev.scopeLabels[s] = devScopeLabel{name: full, site: resourceCallerSite(3)}
}

func (d *devState) scopeInfo(o *Scope) (name, site string) {
	if lbl, ok := d.scopeLabels[o]; ok {
		return lbl.name, lbl.site
	}
	if comp, ok := d.components[o]; ok {
		return comp.name, comp.site
	}
	if o.comp != nil {
		name = compKindName(o.comp.kind)
		if dc := d.comps[o.comp]; dc != nil {
			site = d.siteOf(o.comp, dc)
		}
		return name, site
	}
	if o.rt != nil && o == o.rt.root {
		return "root", ""
	}
	return "scope", ""
}

func (o *Scope) noteCleanupAdded(idx int, kind, detail string, handle any) {
	if o.rt == nil || o.rt.dev == nil {
		return
	}
	d := o.rt.dev
	ownerName, ownerSite := d.scopeInfo(o)
	d.cleanups[o] = append(d.cleanups[o], devCleanup{
		idx:       idx,
		kind:      kind,
		detail:    clip(detail),
		site:      resourceCallerSite(3),
		handle:    handle,
		ownerName: ownerName,
		ownerSite: ownerSite,
	})
}

func (o *Scope) noteImmediateCleanup(kind, detail string) {
	if o.rt == nil || o.rt.dev == nil {
		return
	}
	d := o.rt.dev
	ownerName, ownerSite := d.scopeInfo(o)
	site := resourceCallerSite(3)
	disp := driver.Disposal{
		Owner:     ownerName,
		OwnerSite: ownerSite,
		Reason:    "disposed-owner",
		Site:      site,
		Cleanups: []driver.CleanupEvent{{
			Kind:   kind,
			Detail: clip(detail),
			Site:   site,
		}},
	}
	d.recordDisposal(disp)
}

func (o *Scope) releaseResourceCleanup(handle any) {
	if o == nil || o.rt == nil || o.rt.dev == nil {
		return
	}
	d := o.rt.dev
	if d.disposing != nil {
		// Closing as part of an active disposal: keep it active so disposal attributes it.
		return
	}
	list := d.cleanups[o]
	for i := range list {
		if list[i].handle == handle {
			list[i].closed = true
			return
		}
	}
}

func (o *Scope) beginExplicitDispose() disposalGuard {
	if o.rt == nil || o.rt.dev == nil {
		return disposalGuard{}
	}
	d := o.rt.dev
	name, site := d.scopeInfo(o)
	frame := &disposalFrame{
		root:      o,
		owner:     name,
		ownerSite: site,
		reason:    "dispose",
		site:      resourceCallerSite(2),
		prev:      d.disposing,
	}
	d.disposing = frame
	return disposalGuard{d: d, frame: frame}
}

func (c *computation) beginRerunDispose() disposalGuard {
	if c.rt == nil || c.rt.dev == nil {
		return disposalGuard{}
	}
	d := c.rt.dev
	name, site := d.scopeInfo(&c.Scope)
	frame := &disposalFrame{
		root:      &c.Scope,
		owner:     name,
		ownerSite: site,
		reason:    "rerun",
		prev:      d.disposing,
	}
	d.disposing = frame
	return disposalGuard{d: d, frame: frame}
}

func (g disposalGuard) finish() {
	if g.d == nil || g.frame == nil {
		return
	}
	g.d.disposing = g.frame.prev
	if len(g.frame.cleanups) == 0 {
		return
	}
	g.d.recordDisposal(driver.Disposal{
		Owner:     g.frame.owner,
		OwnerSite: g.frame.ownerSite,
		Reason:    g.frame.reason,
		Site:      g.frame.site,
		Cleanups:  g.frame.cleanups,
	})
}

func (d *devState) recordDisposal(disp driver.Disposal) {
	if len(d.commitDisposals) < maxCommitDisposal {
		d.commitDisposals = append(d.commitDisposals, disp)
	}
	d.disposalHistory = append(d.disposalHistory, disp)
	if len(d.disposalHistory) > maxDisposalLog {
		d.disposalHistory = d.disposalHistory[len(d.disposalHistory)-maxDisposalLog:]
	}
}

func (o *Scope) noteCleanupRunning(idx int) {
	if o.rt == nil || o.rt.dev == nil {
		return
	}
	d := o.rt.dev
	frame := d.disposing
	if frame == nil {
		return
	}
	list := d.cleanups[o]
	for i := range list {
		if list[i].idx == idx {
			if list[i].closed {
				return
			}
			list[i].closed = true
			ev := driver.CleanupEvent{
				Kind:   list[i].kind,
				Detail: list[i].detail,
				Site:   list[i].site,
			}
			if o != frame.root {
				ev.Owner = list[i].ownerName
				ev.OwnerSite = list[i].ownerSite
			}
			frame.cleanups = append(frame.cleanups, ev)
			return
		}
	}
}

func (o *Scope) noteTimersStopped(n int) {
	if n <= 0 || o.rt == nil || o.rt.dev == nil {
		return
	}
	d := o.rt.dev
	frame := d.disposing
	if frame == nil {
		return
	}
	detail := ""
	if n > 1 {
		detail = "×" + strconv.Itoa(n)
	}
	ev := driver.CleanupEvent{Kind: "timer", Detail: detail}
	if o != frame.root {
		ev.Owner, ev.OwnerSite = d.scopeInfo(o)
	}
	frame.cleanups = append(frame.cleanups, ev)
}

func (o *Scope) noteContextCancelled() {
	if o.rt == nil || o.rt.dev == nil {
		return
	}
	d := o.rt.dev
	frame := d.disposing
	if frame == nil {
		return
	}
	ev := driver.CleanupEvent{Kind: "context"}
	if o != frame.root {
		ev.Owner, ev.OwnerSite = d.scopeInfo(o)
	}
	frame.cleanups = append(frame.cleanups, ev)
}

func (d *devState) takeDisposals() []driver.Disposal {
	if len(d.commitDisposals) == 0 {
		return nil
	}
	out := append([]driver.Disposal(nil), d.commitDisposals...)
	d.commitDisposals = d.commitDisposals[:0]
	return out
}

func (d *devState) recentDisposals() []driver.Disposal {
	if len(d.disposalHistory) == 0 {
		return nil
	}
	return append([]driver.Disposal(nil), d.disposalHistory...)
}

func (d *devState) clearDisposals() {
	d.disposalHistory = d.disposalHistory[:0]
}

func (d *devState) ownerResources(o *Scope) []devtools.Resource {
	list := d.cleanups[o]
	if len(list) == 0 {
		return nil
	}
	var out []devtools.Resource
	for _, cl := range list {
		if cl.closed {
			continue
		}
		out = append(out, devtools.Resource{Kind: cl.kind, Detail: cl.detail, Site: cl.site})
	}
	return out
}

// resourceCallerSite finds the first app frame above core and thin wrapper packages (desktop, audio, mobile).
func resourceCallerSite(skip int) string {
	var pcs [causeDepth]uintptr
	n := goruntime.Callers(skip+1, pcs[:])
	frames := goruntime.CallersFrames(pcs[:n])
	last := ""
	for {
		f, more := frames.Next()
		if f.Function == "" {
			break
		}
		if !runsApps(f.Function) && !isThinWrapper(f.Function) && !strings.HasPrefix(f.Function, "runtime.") {
			if strings.HasPrefix(f.Function, nsgoPackage+"tsx") && devtools.ScriptSite != nil {
				if file, line, ok := devtools.ScriptSite(); ok {
					return pathSite(file, line)
				}
			}
			return pathSite(f.File, f.Line)
		}
		if strings.HasPrefix(f.Function, "github.com/nsgo-dev/nsgo") {
			last = strings.TrimPrefix(strings.TrimPrefix(f.Function, nsgoPackage), "github.com/nsgo-dev/")
		}
		if !more {
			break
		}
	}
	if last == "" {
		return "unknown"
	}
	return last
}

// isThinWrapper reports whether fn is in a thin wrapper package over core (desktop, audio, mobile),
// whose functions only forward to Scope methods.
func isThinWrapper(fn string) bool {
	return strings.HasPrefix(fn, nsgoPackage+"desktop.") ||
		strings.HasPrefix(fn, nsgoPackage+"audio.") ||
		strings.HasPrefix(fn, nsgoPackage+"mobile.")
}
