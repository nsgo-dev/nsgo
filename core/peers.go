package core

import "slices"

// peerMarked reports whether n can supply sibling states through its classes.
func (n *node) peerMarked() bool {
	return n.cls != nil && len(n.cls.list.peers) > 0
}

// matchesPeer follows CSS's subsequent-sibling combinator: any matching preceding sibling suffices, without looking inside it or past the parent.
func (t *Tree) matchesPeer(n *node, cond classPeer) bool {
	p := n.nativeParent()
	if p == nil {
		return false
	}
	kids := p.flatChildren()
	i := slices.Index(kids, n)
	if i < 0 {
		return false
	}
	for _, k := range kids[:i] {
		if !k.peerMarked() || !slices.Contains(k.cls.list.peers, cond.name) {
			continue
		}
		shown := t.sharedStates(k)
		var states uint16
		for s, on := range shown.on {
			if on {
				states |= classStates[s]
			}
		}
		if states&cond.states == cond.states && states&cond.notStates == 0 &&
			(cond.data == "" || slices.Contains(shown.data, cond.data)) {
			return true
		}
	}
	return false
}

func (t *Tree) queuePeerSiblings(n *node) {
	if n.peerMarked() {
		t.queuePeerParent(n.nativeParent())
	}
}

// queuePeerParent coalesces changes until the next resolve. A parent is visited once even when several peers change in the same reactive turn.
func (t *Tree) queuePeerParent(p *node) {
	if !t.peerStyled || p == nil || t.refreshingPeers {
		return
	}
	if t.peerParents == nil {
		t.peerParents = make(map[*node]struct{})
	}
	t.peerParents[p] = struct{}{}
	t.changed = true
}

func (t *Tree) updatePeerStyles() {
	if len(t.peerParents) == 0 {
		return
	}
	t.refreshingPeers = true
	defer func() { t.refreshingPeers = false }()
	for p := range t.peerParents {
		delete(t.peerParents, p)
		if p.deleted {
			continue
		}
		for _, k := range p.flatChildren() {
			if k.cls == nil || !k.cls.list.peerDependent {
				continue
			}
			t.resolveClasses(k)
			if t.restyle(k) {
				t.markLayout(k)
			}
		}
	}
}
