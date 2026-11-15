package core

// ScrollAlignment is where Reveal puts a view along a scroll view's axis.
type ScrollAlignment uint8

const (
	// ScrollAlignDefault is the axis's own: ScrollAlignStart vertically,
	// ScrollAlignNearest horizontally.
	ScrollAlignDefault ScrollAlignment = iota
	ScrollAlignStart
	ScrollAlignCenter
	ScrollAlignEnd
	// ScrollAlignNearest scrolls the least that shows it, and not at all
	// while it shows whole.
	ScrollAlignNearest
)

// RevealOptions say where Reveal puts a view along vertical and
// horizontal scroll views, and whether the scrolls animate.
type RevealOptions struct {
	Vertical, Horizontal ScrollAlignment
	Animated             bool
	// Margin is kept clear at the outermost scroll view's edges, for the bars that cover them, such as a find bar over a transcript.
	Margin Edges
}

// reveal is a Reveal waiting for its commit's layout: of n's whole box, or of box in n's frame.
type reveal struct {
	n    *node
	opts RevealOptions
	box  *Bounds
}

// Reveal scrolls every scroll view around the element until it shows,
// innermost first, each clamped to its content: at the commit, with that
// commit's layout, so a view made in the same turn is revealed where it
// lands. The word is NSH's (`reveal`), as GPUI Kit's `reveal_item`; the
// alignments are physical, as NSH's.
func (r *Ref) Reveal(opts RevealOptions) {
	if r == nil || r.n == nil || r.n.deleted {
		return
	}
	r.t.reveals = append(r.t.reveals, reveal{n: r.n, opts: opts})
	r.t.changed = true
}

// RevealBox scrolls every scroll view around the element until box, in the element's frame as TextRangeBoxes gives a text's ranges, shows, as Reveal shows the whole element: a match in a long line of code reaches its column, not only the code's block.
func (r *Ref) RevealBox(box Bounds, opts RevealOptions) {
	if r == nil || r.n == nil || r.n.deleted {
		return
	}
	r.t.reveals = append(r.t.reveals, reveal{n: r.n, opts: opts, box: &box})
	r.t.changed = true
}

// revealViews turns the reveals asked for into scrolls of the scroll
// views around each view, from this commit's layout.
func (t *Tree) revealViews() {
	for _, rv := range t.reveals {
		if !rv.n.deleted {
			box := Bounds{Size: rv.n.frame.Size}
			if rv.box != nil {
				box = *rv.box
			}
			t.revealNode(rv.n, box, rv.opts)
		}
	}
	clear(t.reveals)
	t.reveals = t.reveals[:0]
}

// revealNode scrolls the scroll views around n so box, in n's frame, shows: the box, in each one's content, goes where opts say, and on to the next one out as that scroll leaves it. The outermost one keeps opts.Margin clear around it.
func (t *Tree) revealNode(n *node, box Bounds, opts RevealOptions) {
	var outer *node
	for c := n; c.parent != nil; c = c.parent {
		if p := c.parent; p.scroll != nil && !p.deleted {
			outer = p
		}
	}
	for c := n; c.parent != nil; c = c.parent {
		box.Origin.X += c.frame.Origin.X
		box.Origin.Y += c.frame.Origin.Y
		p := c.parent
		sc := p.scroll
		if sc == nil || p.deleted {
			continue
		}
		// box is in p's content: scroll p so it shows, then follow it
		// into p's own box.
		to := sc.offset
		in := sc.config.Behavior.ContentInset
		size, content := p.frame.Size, sc.content
		shown := box
		if p == outer {
			m := opts.Margin
			shown = Bounds{Origin: Point{X: box.Origin.X - m.Left, Y: box.Origin.Y - m.Top},
				Size: Size{Width: box.Size.Width + m.Left + m.Right, Height: box.Size.Height + m.Top + m.Bottom}}
		}
		if sc.config.Horizontal || sc.config.Zooms() {
			to.X = revealAt(shown.Origin.X, shown.Size.Width, sc.offset.X, size.Width, content.Width, in.Left, in.Right,
				orDefault(opts.Horizontal, ScrollAlignNearest))
		}
		if !sc.config.Horizontal || sc.config.Zooms() {
			to.Y = revealAt(shown.Origin.Y, shown.Size.Height, sc.offset.Y, size.Height, content.Height, in.Top, in.Bottom,
				orDefault(opts.Vertical, ScrollAlignStart))
		}
		if to != sc.offset {
			t.scrollTo(p, to, opts.Animated)
		}
		box.Origin.X -= to.X
		box.Origin.Y -= to.Y
	}
}

func orDefault(a, or ScrollAlignment) ScrollAlignment {
	if a == ScrollAlignDefault {
		return or
	}
	return a
}

// revealAt is the offset along one axis that puts the span at start,
// length long, where align says in a viewport size long scrolled to at,
// its content length long with insets before and after.
func revealAt(start, length, at, size, content, before, after Points, align ScrollAlignment) Points {
	var to Points
	switch align {
	case ScrollAlignStart:
		to = start
	case ScrollAlignCenter:
		to = start + length/2 - size/2
	case ScrollAlignEnd:
		to = start + length - size
	default:
		switch {
		case start >= at && start+length <= at+size:
			return at
		case start < at || length > size:
			to = start
		default:
			to = start + length - size
		}
	}
	return max(min(to, content-size+after), -before)
}
