package core

// Presence is Show whose branches leave with their exits, as gpui-base's
// Presence and Motion's AnimatePresence: when When changes, the branch it
// held stays in the region, before the one built for the new value, until
// every exit its parts registered with Scope.OnExit has called done. A
// branch whose parts registered none goes at once, as Show's does.
//
// A leaving branch never comes back: if When changes back before it is
// gone, a new branch is built beside it. With Wait, the new branch is
// built only once the branches leaving are gone, as AnimatePresence's
// wait mode, so in-flow content never shows twice.
type Presence struct {
	When func() bool
	Then func(cx *Scope) Node
	Else func(cx *Scope) Node
	Wait bool
}

// Render implements Node.
func (p Presence) Render(*Scope) Node { return p }

// PresenceOf is Presence keyed by a comparable value: changing Key sends the
// old Content away through its exits and builds the new Content. With Wait,
// the new content waits until every leaving branch is gone. MaxLeaving bounds
// the branches leaving at once; the oldest leaves immediately past the bound.
type PresenceOf[K comparable] struct {
	Key        func() K
	Content    func(cx *Scope, key K) Node
	Wait       bool
	MaxLeaving int
}

// Render implements Node.
func (p PresenceOf[K]) Render(*Scope) Node { return p }

// presenceKey gives the parts of a Presence's branch the branch.
var presenceKey = NewKey[*presenceBranch]("core.presence", nil)

// presenceBranch is one branch of a Presence: what it built and the
// exits its parts registered.
type presenceBranch struct {
	scope   *Scope
	node    *node
	present *Signal[bool]
	exits   []func(done func())
	left    int  // exits still playing
	gone    bool // disposed
}

// OnExit has exit play when the Presence branch cx is in leaves: exit
// starts what the part does to leave, such as a Motion's exit, and calls
// done once it ends, which it may do at once. The branch goes once every
// exit registered in it has called done. Outside a Presence, exit never
// runs and the part goes with its branch at once.
func (cx *Scope) OnExit(exit func(done func())) {
	if b := presenceKey.Use(cx); b != nil && !b.gone {
		b.exits = append(b.exits, exit)
	}
}

// Present reports whether the Presence branch cx is in is shown, false
// once it started leaving: a part reads it to stop taking presses as it
// goes. Outside a Presence it is true.
func (cx *Scope) Present() bool {
	if b := presenceKey.Use(cx); b != nil {
		return b.present.Get()
	}
	return true
}

// mount implements regionMounter: one render effect builds a branch for
// When's value and sends the one it replaces away through its exits.
// Branches belong to owner, not to the effect, so they outlive its runs.
func (p Presence) mount(t *Tree, region *node, owner *Scope) {
	mountPresence(t, region, owner, func() any { return p.When() }, func(cx *Scope, value any) Node {
		if value.(bool) {
			if p.Then == nil {
				return nil
			}
			return p.Then(cx)
		}
		if p.Else == nil {
			return nil
		}
		return p.Else(cx)
	}, p.Wait, 0)
}

func (p PresenceOf[K]) mount(t *Tree, region *node, owner *Scope) {
	mountPresence(t, region, owner, func() any { return p.Key() }, func(cx *Scope, value any) Node {
		return p.Content(cx, value.(K))
	}, p.Wait, p.MaxLeaving)
}

func mountPresence(t *Tree, region *node, owner *Scope, key func() any, content func(*Scope, any) Node, wait bool, maxLeaving int) {
	when := NewMemoFunc[any](owner, key, func(a, b any) bool { return a == b })
	var cur *presenceBranch
	var leaving []*presenceBranch
	waiting := false // Wait holds the next branch until those leaving go
	order := func() {
		nodes := make([]*node, 0, len(leaving)+1)
		for _, b := range leaving {
			nodes = append(nodes, b.node)
		}
		if cur != nil {
			nodes = append(nodes, cur.node)
		}
		t.setChildren(region, nodes)
	}
	build := func(value any) {
		cur = nil
		if content == nil {
			return
		}
		b := &presenceBranch{scope: &Scope{}}
		owner.adopt(b.scope)
		b.present = NewSignal(b.scope, true)
		b.node = t.newRegion(region, b.scope)
		t.within(b.scope, func() {
			presenceKey.Provide(b.scope, b)
			t.mount(b.scope, b.node, content(b.scope, value))
		})
		cur = b
	}
	var remove func(b *presenceBranch)
	remove = func(b *presenceBranch) {
		if b.gone {
			return
		}
		b.gone = true
		for i, l := range leaving {
			if l == b {
				leaving = append(leaving[:i], leaving[i+1:]...)
				break
			}
		}
		b.scope.Dispose()
		t.discard(b.node)
		if waiting && len(leaving) == 0 {
			waiting = false
			build(when.GetUntracked())
		}
		order()
	}
	leave := func(b *presenceBranch) {
		if len(b.exits) == 0 {
			b.gone = true
			b.scope.Dispose()
			t.discard(b.node)
			return
		}
		for maxLeaving > 0 && len(leaving) >= maxLeaving {
			remove(leaving[0])
		}
		leaving = append(leaving, b)
		b.present.Set(false)
		exits := b.exits
		b.exits = nil
		b.left = len(exits)
		for _, exit := range exits {
			called := false
			t.within(b.scope, func() {
				exit(func() {
					if called || b.gone {
						return
					}
					called = true
					if b.left--; b.left == 0 {
						remove(b)
					}
				})
			})
		}
	}
	first := true
	owner.renderEffect(func() {
		value := when.Get()
		owner.Untrack(func() {
			if first {
				first = false
				build(value)
				order()
				return
			}
			if cur != nil {
				b := cur
				cur = nil
				leave(b)
			}
			if wait && len(leaving) > 0 {
				waiting = true
			} else if !waiting {
				build(value)
			}
			order()
		})
	})
}
