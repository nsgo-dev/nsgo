package ui

import (
	"fmt"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// TabsVariant is a tab list's look.
type TabsVariant uint8

const (
	// TabsSegmented sets the selected tab on a raised thumb in a recessed
	// capsule: the default.
	TabsSegmented TabsVariant = iota
	// TabsUnderline marks the selected tab with an accent underline.
	TabsUnderline
)

// tabsSlide is how the indicator moves to the selected tab.
var tabsSlide = core.Transition{Duration: 180 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionLayout}

// Tabs picks one of Options: a tab list whose content the app shows beside
// it, or a segmented control. One indicator marks the selected tab and
// slides to the next, resizing to it.
type Tabs[T comparable] struct {
	Variant TabsVariant
	Options []Option[T]
	// OptionsFunc, when set, returns the options in place of Options and updates tabs in place when the values it reads change, keeping the selected tab and focus where their values remain offered.
	OptionsFunc func() []Option[T]
	Value       func() T
	// OnChange receives the value of a pressed tab other than the selected
	// one.
	OnChange func(T)
	// Equal fills the width with tabs of one width, instead of each
	// fitting its content.
	Equal bool
	Size  Size
	// Label names the list for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (t Tabs[T]) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	underline := t.Variant == TabsUnderline
	options := func() []Option[T] {
		if t.OptionsFunc != nil {
			return t.OptionsFunc()
		}
		return t.Options
	}
	selected := func() int {
		if t.Value == nil {
			return 0
		}
		v := t.Value()
		for i, o := range options() {
			if o.Value == v {
				return i
			}
		}
		return -1
	}
	list := core.Style{Direction: core.Row, Gap: 4}
	height, text, pad := t.Size.Height(), t.Size.text(), t.Size.inset()-2
	tab := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
		Padding: core.Symmetric(pad-4, pad), CornerRadius: RadiusControl, FontSize: text.FontSize, LineHeight: text.LineHeight, FontWeight: Regular}
	if !underline {
		list.Padding, list.Gap, list.CornerRadius = core.All(2), 2, RadiusFull
		tab.Height, tab.Padding, tab.CornerRadius = core.Pt(height), core.Symmetric(0, pad), height/2
	}
	if t.Equal {
		// A grid's tracks are one width whether the list fills its parent or
		// fits the widest tab.
		list.Display = core.DisplayGrid
	} else if !underline {
		list.AlignSelf = core.AlignStart
	}
	width := core.NewSignal(cx, core.Points(0))
	frames := map[T]*core.Signal[core.Bounds]{}
	frame := func(value T) *core.Signal[core.Bounds] {
		if f := frames[value]; f != nil {
			return f
		}
		f := core.NewSignal(cx, core.Bounds{})
		frames[value] = f
		return f
	}
	// Only a change of tab slides: the indicator follows a tab that a
	// resize moves at once.
	sliding := core.NewSignal(cx, false)
	last := core.Untracked(cx, selected)
	cx.Effect(func() {
		if i := selected(); i != last {
			last = i
			sliding.Set(true)
		}
	})
	env := cx.Env()
	indicator := core.View{PointerEvents: core.PointerEventsNone, OnTransitionEnd: func() { sliding.Set(false) }, StyleFunc: func() core.Style {
		var f core.Bounds
		if i := selected(); i >= 0 {
			o := options()[i]
			f = frame(o.Value).Get()
		}
		left := f.Origin.X
		// Right to left, Left is the distance from the list's right.
		if env.RTL() {
			left = width.Get() - f.Origin.X - f.Size.Width
		}
		s := core.Style{Position: core.Absolute}
		if sliding.Get() {
			s.Transition = tabsSlide
		}
		if underline {
			const inset, height = 6, 2
			s.Left, s.Top, s.Width, s.Height = core.Pt(left+inset), core.Pt(f.Origin.Y+f.Size.Height-height), core.Pt(max(0, f.Size.Width-2*inset)), core.Pt(height)
			s.CornerRadius, s.Background = height/2, colors.Accent()
			return s
		}
		s.Left, s.Top, s.Width, s.Height = core.Pt(left), core.Pt(f.Origin.Y), core.Pt(f.Size.Width), core.Pt(f.Size.Height)
		s.CornerRadius = f.Size.Height / 2
		return FinOf(cx).Pill().Over(s)
	}}
	tabLook := func() core.Style {
		// Selection keeps the weight, so it never moves the labels.
		return core.Style{Color: Alpha(colors.Text(), 0.6), Selected: &core.Style{Color: colors.Text()}, FocusVisible: FocusRing(cx, -2), Disabled: Dimmed()}
	}
	tabs := []core.Node{indicator, core.Each(options, func(o Option[T]) T { return o.Value }, func(rowCx *core.Scope, item func() Option[T], index func() int) core.Node {
		o := item()
		f := frame(o.Value)
		children := []core.Node{o.Content}
		if o.Content == nil {
			children = nil
			if o.Icon != nil {
				children = append(children, Icon(o.Icon, t.Size.mark(), Inherited))
			}
			if o.Label != "" {
				children = append(children, core.Text{ContentFunc: func() string { return item().Label }, MaxLines: 1})
			}
		}
		return base.Tab{Selected: func() bool { return selected() == index() }, Disabled: func() bool { return item().Disabled != nil && item().Disabled() },
			OnPress: func(core.PressEvent) {
				if t.OnChange != nil && core.Untracked(cx, selected) != index() {
					t.OnChange(item().Value)
				}
			},
			OnLayout: f.Set, Style: tab, StyleFunc: tabLook, Children: children, LabelFunc: func() string { return item().Label }, TestID: o.TestID}
	})}
	return base.Tabs{Style: list, StyleFunc: func() core.Style {
		style := core.Style{}
		if t.Equal {
			style.GridTemplateColumns = core.GridTracks(fmt.Sprintf("repeat(%d, minmax(0, 1fr))", max(1, len(options()))))
		}
		if underline {
			return style.Merge(t.Style)
		}
		return core.Style{Background: Alpha(colors.Text(), 0.06)}.Merge(style).Merge(t.Style)
	}, OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: tabs, Label: t.Label, TestID: t.TestID}
}
