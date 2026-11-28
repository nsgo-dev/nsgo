package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base/toast"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// ToastType picks a toast's colored status icon; ToastPlain shows none.
type ToastType uint8

const (
	ToastPlain ToastType = iota
	ToastInfo
	ToastSuccess
	ToastWarning
	ToastError
)

// ToastContent is a toast: a Title, a quieter Description and, with
// Action, a button that runs OnAction and closes it. Leading replaces the
// Type's status icon, as with an image or a spinner. Duration is how long
// it stays, 5 s when 0 and until it is dismissed when negative. OnClose
// runs once it has gone, whatever closed it. A toast with the ID of one
// shown replaces it.
type ToastContent struct {
	ID          string
	Type        ToastType
	Title       string
	Description string
	Leading     core.Node
	// Trailing is content after Action and before the optional close button.
	Trailing core.Node
	Action   string
	OnAction func()
	// OnPress makes the toast surface a button and dismisses it after the callback. Action, Trailing and close-button children keep their presses.
	OnPress func()
	// NoClose omits this toast's close button. Swipe, Escape and its timeout still dismiss it.
	NoClose bool
	OnClose func()
	// Priority orders waiting toasts: PriorityDefault (zero) gives an
	// error or a toast that stays until it is dismissed PriorityHigh and
	// others PriorityNormal; PriorityNormal keeps even an error or sticky
	// toast in turn.
	Priority toast.Priority
	Duration time.Duration
	// Style and StyleFunc style this toast's surface over the stack's style.
	// StyleFunc follows the signals it reads and merges over Style.
	Style     core.Style
	StyleFunc func() core.Style
	shown     uint64 // which Show it is, by which a replacement is told from the toast it replaces
}

type toastEntry = toast.Entry[string, ToastContent]

const (
	toastLimit = 3                     // how many show at once by default
	toastTick  = 50 * time.Millisecond // how often the lifecycle advances while one shows
)

// Toaster keeps the app's toasts: the app makes one near its root with
// NewToaster, shows and dismisses toasts through it from anywhere, and
// places a Toasts of it once over its content. NSGo's ToastManager keeps
// their lifecycle; a toast's timeout pauses while the stack is open
// under the pointer or a finger, and while the app is in the background.
// Limit is how many show at once, 3 when 0: the oldest show, then the
// rest in turn, an error or a running job's progress first.
type Toaster struct {
	Limit    int
	cx       *core.Scope
	manager  toast.Manager[string, ToastContent]
	motion   toast.Motion
	shown    *core.Signal[[]toastEntry]
	waiting  *core.Signal[int]
	expanded *core.Signal[bool]
	ticking  bool
	count    int
	shows    uint64
}

// NewToaster makes a Toaster where cx is.
func NewToaster(cx *core.Scope) *Toaster {
	return &Toaster{cx: cx, shown: core.NewSignalFunc(cx, []toastEntry(nil), func(a, b []toastEntry) bool { return false }),
		waiting: core.NewSignal(cx, 0), expanded: core.NewSignal(cx, false)}
}

// applyMotion writes t.motion to t.manager, zeroing enter and exit
// durations when reduced motion is active.
func (t *Toaster) applyMotion(reduce bool) {
	m := t.motion
	if reduce {
		m.Duration = -1
		m.ExitDuration = -1
	}
	t.manager.Motion = m
}

func (t *Toaster) syncMotion() {
	t.applyMotion(core.Untracked(t.cx, t.cx.Env().ReduceMotion))
}

// Show shows c and returns its ID, made up when c has none.
func (t *Toaster) Show(c ToastContent) string {
	if c.ID == "" {
		t.count++
		c.ID = fmt.Sprintf("toast-%d", t.count)
	}
	if c.Duration == 0 {
		c.Duration = 5 * time.Second
	}
	t.shows++
	c.shown = t.shows
	priority := c.Priority
	if priority == toast.PriorityDefault {
		if c.Type == ToastError || c.Duration < 0 {
			priority = toast.PriorityHigh
		} else {
			priority = toast.PriorityNormal
		}
	}
	t.manager.Limit = t.limit()
	t.syncMotion()
	if old, ok := t.manager.PushPriority(c.ID, c, max(c.Duration, 0), priority, t.cx.Now()); ok && old.OnClose != nil {
		old.OnClose()
	}
	t.run()
	return c.ID
}

// Waiting returns how many toasts wait behind the ones showing, as a
// count beside the stack shows; it follows the queue, as shown does.
func (t *Toaster) Waiting() int { return t.waiting.Get() }

// limit is how many toasts show at once.
func (t *Toaster) limit() int {
	if t.Limit == 0 {
		return toastLimit
	}
	return t.Limit
}

// Dismiss closes the toast id.
func (t *Toaster) Dismiss(id string) {
	t.manager.Limit = t.limit()
	if t.manager.Dismiss(id, t.cx.Now()) {
		t.run()
	}
}

// DismissAll closes every toast.
func (t *Toaster) DismissAll() {
	t.manager.Limit = t.limit()
	if len(t.manager.DismissAll(t.cx.Now())) > 0 {
		t.run()
	}
}

// visible is the toasts to show.
func (t *Toaster) visible() []toastEntry {
	t.manager.Limit = t.limit()
	return t.manager.Visible()
}

// waitingCount is how many wait behind them.
func (t *Toaster) waitingCount() int {
	t.manager.Limit = t.limit()
	return t.manager.Waiting()
}

func (t *Toaster) run() {
	t.shown.Set(t.visible())
	t.waiting.Set(t.waitingCount())
	if !t.ticking {
		t.ticking = true
		// A timer belongs to the scope that starts it: posted, it is the
		// Toaster's, not that of a dialog showing a toast as it closes.
		t.cx.Post(func() { t.cx.AfterFunc(toastTick, t.tick) })
	}
}

func (t *Toaster) tick() {
	t.manager.Limit = t.limit()
	t.syncMotion()
	paused := t.expanded.GetUntracked() || core.Untracked(t.cx, t.cx.Env().Lifecycle) == core.LifecycleBackground
	a := t.manager.Advance(t.cx.Now(), paused)
	if a.Changed {
		t.shown.Set(t.visible())
		t.waiting.Set(t.manager.Waiting())
	}
	for _, gone := range a.Removed {
		if gone.Value.OnClose != nil {
			gone.Value.OnClose()
		}
	}
	if t.manager.Len() == 0 {
		t.ticking = false
		return
	}
	t.cx.AfterFunc(toastTick, t.tick)
}

// ToastPlacement is the edge and side of the window toasts stack at;
// the newest is nearest the edge. The zero value is the top center.
type ToastPlacement uint8

const (
	ToastTop ToastPlacement = iota
	ToastTopStart
	ToastTopEnd
	ToastBottom
	ToastBottomStart
	ToastBottomEnd
	// ToastStart and ToastEnd are the middle of the window's sides.
	ToastStart
	ToastEnd
)

func (p ToastPlacement) top() bool { return p <= ToastTopEnd }

func (p ToastPlacement) side() bool { return p >= ToastStart }

// anchor and align are the stack's anchor and where it sits across.
func (p ToastPlacement) anchor() (toast.Anchor, core.Align) {
	switch p {
	case ToastTopStart:
		return toast.AnchorTopLeft, core.AlignStart
	case ToastTopEnd:
		return toast.AnchorTopRight, core.AlignEnd
	case ToastBottom:
		return toast.AnchorBottomCenter, core.AlignCenter
	case ToastBottomStart:
		return toast.AnchorBottomLeft, core.AlignStart
	case ToastBottomEnd:
		return toast.AnchorBottomRight, core.AlignEnd
	case ToastStart:
		return toast.AnchorLeftCenter, core.AlignStart
	case ToastEnd:
		return toast.AnchorRightCenter, core.AlignEnd
	}
	return toast.AnchorTopCenter, core.AlignCenter
}

// ToastSwipe is the way toasts are swiped away. The zero value is toward
// the nearest edge of the window.
type ToastSwipe uint8

const (
	ToastSwipeEdge ToastSwipe = iota
	ToastSwipeLeft
	ToastSwipeRight
	ToastSwipeUp
	ToastSwipeDown
	// ToastSwipeOff keeps them from being swiped.
	ToastSwipeOff
)

// direction is the swipe of toasts at p, in a layout right to left or not.
func (s ToastSwipe) direction(p ToastPlacement, rtl bool) toast.SwipeDirection {
	switch s {
	case ToastSwipeLeft:
		return toast.SwipeLeft
	case ToastSwipeRight:
		return toast.SwipeRight
	case ToastSwipeUp:
		return toast.SwipeUp
	case ToastSwipeDown:
		return toast.SwipeDown
	case ToastSwipeOff:
		return toast.SwipeOff
	}
	switch {
	case p.side() && (p == ToastStart) != rtl:
		return toast.SwipeLeft
	case p.side():
		return toast.SwipeRight
	case p.top():
		return toast.SwipeUp
	}
	return toast.SwipeDown
}

// Toasts is ui's toasts, a Toaster's, over the window inside the safe
// area: glass cards up to 360 points wide, the newest in front and older
// ones tucked behind it, fanning out under the pointer or a finger.
// Reverse puts the newest farthest from the edge, as a list growing away
// from it. Swiping one away, Swipe's way, or Escape closes it. Inset adds
// room at the placement's edge, as for a shell's header. CloseLabel names
// each toast's close button for assistive technology.
type Toasts struct {
	Toaster    *Toaster
	Placement  ToastPlacement
	Reverse    bool
	Swipe      ToastSwipe
	Inset      core.Points
	CloseLabel string
	// Style and StyleFunc style every toast's surface. StyleFunc merges
	// over Style, and each ToastContent's styles merge over both.
	Style     core.Style
	StyleFunc func() core.Style
	// NoClose omits the close button from every toast in the stack, whatever its ToastContent.NoClose.
	NoClose bool
	// Polite reads every toast as a status, a polite live region, rather
	// than an alert; an error keeps its alert.
	Polite bool
	// Motion is the toasts' entering and leaving, and the stack's
	// springs, Sonner's where a field is zero.
	Motion toast.Motion
	TestID string
}

// Render implements core.Node.
func (ts Toasts) Render(cx *core.Scope) core.Node {
	t := ts.Toaster
	t.motion = ts.Motion
	cx.Effect(func() {
		t.applyMotion(cx.Env().ReduceMotion())
	})
	anchor, align := ts.Placement.anchor()
	// An overlay sizes to its content: the window's width gives align room.
	place := core.Style{Left: core.Pt(0), Width: core.Pct(100), AlignItems: align, Padding: core.All(16)}
	switch {
	case ts.Placement.side():
		place.Top, place.Height, place.JustifyContent = core.Pt(0), core.Pct(100), core.JustifyCenter
		place.PaddingEnv = core.EnvEdges{Top: core.EnvSafeArea, Bottom: core.EnvSafeArea}
	case ts.Placement.top():
		place.Top, place.Padding.Top, place.PaddingEnv = core.Pt(0), 16+ts.Inset, core.EnvEdges{Top: core.EnvSafeArea}
	default:
		place.Bottom, place.Padding.Bottom, place.PaddingEnv = core.Pt(0), 16+ts.Inset, core.EnvEdges{Bottom: core.EnvSafeArea}
	}
	motion := ts.Motion
	if motion.CollapsedPeek == 0 {
		motion.CollapsedPeek = 14
	}
	if motion.ExpandedGap == 0 {
		motion.ExpandedGap = 8
	}
	return core.Overlay{PointerEvents: core.PointerEventsBoxNone, Style: place,
		Children: []core.Node{toast.Stack[toastEntry, string]{TestID: ts.TestID, Placement: anchor, Reverse: ts.Reverse,
			Motion: motion,
			Toasts: t.shown.Get, Key: func(e toastEntry) string { return e.ID },
			Toast: func(cx *core.Scope, e func() toastEntry) core.Node {
				return ts.item(cx, e)
			},
			Style:            core.Style{Width: core.Pt(360), MaxWidth: core.Pct(100)},
			OnExpandedChange: t.expanded.Set,
			OnCancel: func() {
				shown := t.shown.GetUntracked()
				for i := len(shown) - 1; i >= 0; i-- {
					if shown[i].Status != toast.Ending {
						t.Dismiss(shown[i].ID)
						return
					}
				}
			}}}}
}

// item is one toast.
func (ts Toasts) item(cx *core.Scope, e func() toastEntry) core.Node {
	t := ts.Toaster
	colors := ColorsOf(cx)
	id := core.Untracked(cx, e).ID
	motion := ts.Motion
	if motion.Duration == 0 {
		motion.Duration = toast.SonnerMotion().Duration
	} else if motion.Duration < 0 {
		motion.Duration = 0
	}
	if motion.ExitDuration == 0 {
		motion.ExitDuration = toast.SonnerMotion().ExitDuration
	} else if motion.ExitDuration < 0 {
		motion.ExitDuration = 0
	}
	// The content, which changes only when a toast of the same ID replaces it: the entry changes whenever any toast's status does, and building the content again then would restart a leading spinner and drop a focused action.
	content := core.NewMemoFunc(cx, func() ToastContent { return e().Value }, func(a, b ToastContent) bool { return a.shown == b.shown })
	label := func() string {
		c := content.Get()
		return strings.Join(slices.DeleteFunc([]string{c.Title, c.Description}, func(s string) bool { return s == "" }), ". ")
	}
	leading := core.Dynamic(func(*core.Scope) core.Node {
		if lead := toastLeading(content.Get()); lead != nil {
			return core.View{Style: core.Style{FlexShrink: core.NoShrink}, Children: []core.Node{lead}}
		}
		return nil
	})
	// custom is the app's surface style, the stack's under the toast's own.
	custom := func() core.Style {
		c := content.Get()
		style := ts.Style
		if ts.StyleFunc != nil {
			style = style.Merge(ts.StyleFunc())
		}
		style = style.Merge(c.Style)
		if c.StyleFunc != nil {
			style = style.Merge(c.StyleFunc())
		}
		return style
	}
	// The title and description take the surface's Color when the app sets one, so text stays legible on a surface of the app's own, such as a dark pill.
	titleColor := func() core.Color { return cmp.Or(custom().Color, colors.Text()) }
	detailColor := func() core.Color {
		if c := custom().Color; c != 0 {
			return Alpha(c, 0.7)
		}
		return colors.Muted()
	}
	textBody := core.Dynamic(func(*core.Scope) core.Node {
		c := content.Get()
		texts := []core.Node{core.Text{Content: c.Title, Style: TypeLabel, StyleFunc: Colored(titleColor)}}
		if c.Description != "" {
			texts = append(texts, core.Text{Content: c.Description, Style: TypeCaption, StyleFunc: Colored(detailColor)})
		}
		return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: core.Shrink(1), MinWidth: core.Pt(0), Gap: 2}, Children: texts}
	})
	action := core.Dynamic(func(*core.Scope) core.Node {
		c := content.Get()
		if c.Action == "" {
			return nil
		}
		return Button{Text: c.Action, Variant: ButtonSecondary, TestID: id + "-action", OnPress: func() {
			if c.OnAction != nil {
				c.OnAction()
			}
			t.Dismiss(id)
		}}
	})
	children := []core.Node{leading, textBody, action}
	if content.Get().Trailing != nil {
		children = append(children, content.Get().Trailing)
	}
	if !content.Get().NoClose && !ts.NoClose {
		children = append(children, IconButton{Icon: lucide.X, Label: ts.CloseLabel, Size: 24, TestID: id + "-close", OnPress: func() { t.Dismiss(id) }})
	}
	swipe := ts.Swipe.direction(ts.Placement, core.Untracked(cx, cx.Env().RTL))
	var enter core.Transform
	switch {
	case ts.Placement.side() && swipe == toast.SwipeRight:
		enter.TranslateX = 16
	case ts.Placement.side():
		enter.TranslateX = -16
	case ts.Placement.top():
		enter.TranslateY = -16
	default:
		enter.TranslateY = 16
	}
	var onPress func()
	if content.Get().OnPress != nil {
		onPress = func() {
			c := content.Get()
			if c.OnPress != nil {
				c.OnPress()
			}
			t.Dismiss(id)
		}
	}
	return toast.Toast{TestID: id, SwipeDirection: swipe,
		PoliteFunc:       func() bool { return ts.Polite && content.Get().Type != ToastError },
		LabelFunc:        label,
		Status:           func() toast.Status { return e().Status },
		OnDismissRequest: func() { t.Dismiss(id) },
		OnPress:          onPress,
		StartingStyle:    core.Style{Opacity: core.Alpha(0), Transform: enter},
		EndingStyle:      core.Style{Opacity: core.Alpha(0), Transform: core.Transform{Scale: 0.96}},
		SwipingStyle:     core.Style{Transition: core.Transition{}},
		StyleFunc: func() core.Style {
			c := content.Get()
			align := core.AlignCenter
			if c.Description != "" {
				align = core.AlignStart
			}
			glass := FinOf(cx).Glass()
			d := motion.Duration
			if e().Status == toast.Ending {
				d = motion.ExitDuration
			}
			if cx.Env().ReduceMotion() {
				d = 0
			}
			return core.Style{Direction: core.Row, AlignItems: align, Gap: 12,
				Padding: core.Edges{Top: 12, Right: 8, Bottom: 12, Left: 16}, CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous,
				BorderWidth: 1, BorderColor: colors.Border(), Background: Alpha(colors.Background(), 0.92), Color: colors.Text(),
				Shadows: glass.Shadows, BackdropFilters: glass.Backdrop,
				Transition: core.Transition{Duration: d, Easing: core.EaseOut}}.Merge(custom())
		},
		Children: children}
}

// toastLeading is the app's leading content or the type's status icon.
func toastLeading(c ToastContent) core.Node {
	if c.Leading != nil {
		return c.Leading
	}
	var name func() *core.SVG
	var tint func(Colors) func() core.Color
	switch c.Type {
	case ToastInfo:
		name, tint = lucide.Info, func(c Colors) func() core.Color { return c.Accent }
	case ToastSuccess:
		name, tint = lucide.CircleCheck, func(c Colors) func() core.Color { return c.Up }
	case ToastWarning:
		name, tint = lucide.TriangleAlert, func(c Colors) func() core.Color { return c.Warn }
	case ToastError:
		name, tint = lucide.CircleX, func(c Colors) func() core.Color { return c.Danger }
	default:
		return nil
	}
	return core.Func(func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Height: core.Pt(18), JustifyContent: core.JustifyCenter},
			Children: []core.Node{Icon(name, 18, tint(ColorsOf(cx)))}}
	})
}
