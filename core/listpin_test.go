package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// animatedScroller is a platform scroll view for these tests. An animated ItemScrollTo eases from where the view is to its target over a number of frames, reporting each offset with every scroll it was sent counted as applied, as UIKit's setContentOffset:animated: and Android's and macOS's eased scrolls do; another one on the way starts again from where the view is.
//
// An ItemScrollBy on the way is carried along to the ease's target, as ItemScrollBy says, unless drops, when it moves only the offset of that moment, which the ease's next frame replaces, or cancels, when it stops the ease where it is. lags reports a frame of the ease before the platform applies the batch the core sent meanwhile, so the report predates the core's latest scrolls.
type animatedScroller struct {
	offset        Points
	from, to      Points
	frame, frames int
	curve         func(float64) float64
	gliding       bool
	drops         bool
	cancels       bool
	lags          bool
	applied       uint32
}

func (p *animatedScroller) apply(b *driver.MountBatch, id NodeID, limit Points) {
	clamp := func(y Points) Points { return min(max(y, 0), limit) }
	for _, it := range b.Items {
		if it.ID != id {
			continue
		}
		switch it.Kind {
		case driver.ItemScrollTo:
			p.applied++
			if it.Animated {
				p.from, p.to, p.frame, p.gliding = p.offset, clamp(it.Point.Y), 0, true
			} else {
				p.offset, p.gliding = clamp(it.Point.Y), false
			}
		case driver.ItemScrollBy:
			p.applied++
			p.offset += it.Point.Y
			switch {
			case !p.gliding || p.drops:
			case p.cancels:
				p.gliding = false
			default:
				p.from += it.Point.Y
				p.to += it.Point.Y
			}
		}
	}
}

// step moves an ease one frame and reports where it is.
func (p *animatedScroller) step(h *host, id NodeID) {
	p.frame++
	p.offset = p.from + (p.to-p.from)*Points(p.curve(float64(p.frame)/float64(p.frames)))
	if p.frame >= p.frames {
		p.offset, p.gliding = p.to, false
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: id, Point: Point{Y: p.offset}, Count: p.applied})
}

// An animated ScrollToIndexAt to a row hundreds of rows from the measured ones, all shorter than the estimate, ends with the row at viewPosition and the pin done, on platforms whose eases carry a scroll by along and on those whose eases do not: the rows measured on the way, above the viewport and in it, move the target while the platform eases to it (#2719).
func TestListAnimatedPinReachesAFarRow(t *testing.T) {
	const count, viewport = 846, 800
	height := func(i int) Points { return Points(42 + i*7%22) }
	easeOut := func(t float64) float64 { return 1 - (1-t)*(1-t)*(1-t) }
	easeInOut := func(t float64) float64 { return t * t * (3 - 2*t) }
	platforms := []struct {
		name     string
		scroller animatedScroller
	}{
		{"carries", animatedScroller{frames: 18, curve: easeOut}},
		{"drops", animatedScroller{frames: 18, curve: easeInOut, drops: true}},
		{"cancels", animatedScroller{frames: 18, curve: easeInOut, cancels: true}},
		{"lags", animatedScroller{frames: 18, curve: easeInOut, cancels: true, lags: true}},
	}
	for _, pl := range platforms {
		for _, tc := range []struct{ from, to int }{{74, 406}, {74, 845}, {600, 20}} {
			t.Run(fmt.Sprintf("%s/%d-%d", pl.name, tc.from, tc.to), func(t *testing.T) {
				ref := &Ref{}
				var frame *Signal[int]
				bound := map[int]int{}
				h := newHost(t, Size{Width: 400, Height: viewport}, func(cx *Scope) Node {
					frame = NewSignal(cx, 0)
					return List{Ref: ref, TestID: "list", Style: Style{FlexGrow: 1},
						Count: func() int { return count }, EstimatedSize: 64,
						Row: func(cx *Scope, r ListRow) Node {
							return View{StyleFunc: func() Style {
								// A row takes its size two frames after it binds, as text laid out again does, by which time a fast scroll has left it above the viewport.
								i, f := r.Index(), frame.Get()
								if _, ok := bound[i]; !ok {
									bound[i] = f
								}
								if f-bound[i] < 2 {
									return Style{Height: Pt(height(i) - 8)}
								}
								return Style{Height: Pt(height(i))}
							}}
						}}
				})
				h.commit()
				list := h.find("list")
				ln := list.list
				p := pl.scroller
				var sent []string
				run := func() {
					quiet := 0
					for range 2000 {
						h.advanceFrame()
						frame.Set(frame.Get() + 1)
						b := h.commit()
						for _, it := range b.Items {
							if it.ID == list.id && (it.Kind == driver.ItemScrollTo || it.Kind == driver.ItemScrollBy) {
								sent = append(sent, fmt.Sprintf("%v %v (animated %v) at %v", it.Kind, it.Point.Y, it.Animated, p.offset))
							}
						}
						lagged := p.lags && p.gliding
						if lagged {
							p.step(h, list.id)
						}
						p.apply(b, list.id, max(list.scroll.content.Height-viewport, 0))
						if len(b.Items) > 0 {
							h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: list.id, Point: Point{Y: p.offset}, Count: p.applied})
						}
						switch {
						case p.gliding:
							if !lagged {
								p.step(h, list.id)
							}
						case len(b.Items) > 0 || h.tree.Pacing():
							quiet = 0
						default:
							if quiet++; quiet == 10 {
								return
							}
						}
					}
					t.Fatal("the list never came to rest")
				}
				ref.ScrollToIndexAt(tc.from, 0, 0, false)
				run()
				sent = nil
				ref.ScrollToIndexAt(tc.to, 0.3, 0, true)
				run()
				want := 0.3 * (viewport - height(tc.to))
				if tc.to == count-1 {
					// The last row goes no further than the viewport's end.
					want = viewport - height(tc.to)
				}
				c := ln.byIndex[tc.to]
				if c == nil {
					t.Fatalf("row %d is not bound at rest, the platform at %v; scrolls sent:\n%s", tc.to, p.offset, strings.Join(sent, "\n"))
				}
				if got := c.node.frame.Origin.Y - p.offset; abs32(got-want) > 1 {
					t.Errorf("row %d is %v from the viewport's start, want %v; scrolls sent:\n%s", tc.to, got, want, strings.Join(sent, "\n"))
				}
				if ln.st.pin != nil {
					t.Errorf("at rest the pin %+v is still on", *ln.st.pin)
				}
			})
		}
	}
}
