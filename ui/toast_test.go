package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base/toast"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestToasts shows toasts read as alerts, runs an action and closes its
// toast, closes one with its close button, and lets one time out.
func TestToasts(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		undone := false
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, TestID: "toasts", CloseLabel: "Close"}
		})
		toaster.Show(toastOf("saved", ToastSuccess, "Saved", "To your library", "Undo", func() { undone = true }))
		toaster.Show(toastOf("sent", ToastPlain, "Sent", "", "", nil))
		app.Advance(time.Second)
		if v := app.Find("saved"); v.Props.Role != core.RoleAlert || v.Props.Label != "Saved. To your library" {
			t.Fatalf("a toast reads %q as %v", v.Props.Label, v.Props.Role)
		}
		app.Press("saved-action")
		app.Advance(time.Second)
		if !undone || app.Exists("saved") {
			t.Fatal("the action did not run and close its toast")
		}
		app.Press("sent-close")
		app.Advance(time.Second)
		if app.Exists("sent") {
			t.Fatal("the close button left its toast")
		}
		toaster.Show(ToastContent{ID: "brief", Title: "Brief", Duration: 2 * time.Second})
		app.Advance(time.Second)
		app.Find("brief")
		app.Advance(2 * time.Second)
		if app.Exists("brief") {
			t.Fatal("a toast stayed past its duration")
		}
	})
}

func TestToastContentOptions(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		pressed := false
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, TestID: "toasts", CloseLabel: "Close"}
		})
		toaster.Show(ToastContent{ID: "plain", Title: "Plain", NoClose: true, Duration: -1})
		toaster.Show(ToastContent{ID: "open", Title: "Open", Description: "Finished", Trailing: core.Text{Content: "Ready", TestID: "trailing"}, OnPress: func() { pressed = true }, Duration: -1})
		app.Advance(time.Second)
		if app.Exists("plain-close") {
			t.Fatal("NoClose toast still has a close button")
		}
		if !app.Exists("text:Ready") {
			t.Fatal("trailing toast content is missing")
		}
		if v := app.Find("open"); v.Props.Role != core.RoleButton || v.Props.Label != "Open. Finished" {
			t.Fatalf("pressable toast props = role %v label %q", v.Props.Role, v.Props.Label)
		}
		app.Press("open")
		app.Advance(time.Second)
		if !pressed || app.Exists("open") {
			t.Fatalf("whole-toast press: callback=%v exists=%v", pressed, app.Exists("open"))
		}
	})
}

func TestToastSurfaceStyles(t *testing.T) {
	var plain, styled *Toaster
	var stackFill *core.Signal[core.Color]
	var radius *core.Signal[core.Points]
	stackBackground, itemBackground := core.RGB(0x444466), core.RGB(0x121522)
	itemBorder := core.RGB(0xbd365c)
	app, palette := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		plain, styled = NewToaster(cx), NewToaster(cx)
		stackFill = core.NewSignal(cx, stackBackground)
		radius = core.NewSignal(cx, core.Points(24))
		return core.View{Children: []core.Node{
			Toasts{Toaster: plain, NoClose: true},
			Toasts{Toaster: styled, Placement: ToastBottom, NoClose: true,
				Style:     core.Style{Background: core.RGB(0x334422), BorderWidth: 3, CornerRadius: 8, Color: core.RGB(0xf0f0f0)},
				StyleFunc: func() core.Style { return core.Style{Background: stackFill.Get()} }},
		}}
	}, nsgotest.ManualFrames())
	plain.Show(ToastContent{ID: "plain", Title: "Saved", Duration: -1})
	plain.Show(ToastContent{ID: "custom", Title: "Copied", Duration: -1,
		Style: core.Style{Background: itemBackground, BorderColor: itemBorder, CornerRadius: 22, Padding: core.All(24), Color: core.RGB(0xffffff)}})
	styled.Show(ToastContent{ID: "inherited", Title: "Uploading", Description: "3 files", Duration: -1})
	styled.Show(ToastContent{ID: "overridden", Title: "Synced", Duration: -1,
		Style:     core.Style{Background: itemBackground, CornerRadius: 20},
		StyleFunc: func() core.Style { return core.Style{CornerRadius: radius.Get()} }})
	app.Settle()
	if v := app.FindMounted("id:custom"); v.Style.Opacity != 0 || v.Style.Transform.TranslateY != -16 {
		t.Fatalf("custom toast started at opacity %v and translation %v", v.Style.Opacity, v.Style.Transform.TranslateY)
	}
	app.Advance(time.Second)
	for _, want := range []struct {
		id            string
		fill, border  core.Color
		width, radius core.Points
	}{
		{"plain", Alpha(palette.Background, 0.92), palette.Border, 1, RadiusCard},
		{"custom", itemBackground, itemBorder, 1, 22},
		{"inherited", stackBackground, palette.Border, 3, 8},
		{"overridden", itemBackground, palette.Border, 3, 24},
	} {
		s := app.Find(want.id).Style
		if s.Background != want.fill || s.BorderColor != want.border || s.BorderWidth != want.width || s.CornerRadius != want.radius {
			t.Errorf("%s surface: background %x, border %x at %v pt, radius %v", want.id, s.Background, s.BorderColor, s.BorderWidth, s.CornerRadius)
		}
		// The motion is Sonner's entering duration, the app's Motion's.
		if s.Transition.Duration != toast.SonnerMotion().Duration || s.Opacity != 1 {
			t.Errorf("%s lost its transition or starting phase did not end", want.id)
		}
	}
	if got := app.Find("text:Copied").TextStyle.Color; got != core.RGB(0xffffff) {
		t.Errorf("the toast's title is %x, want the surface's own color", got)
	}
	if title, detail := app.Find("text:Uploading").TextStyle.Color, app.Find("text:3 files").TextStyle.Color; title != core.RGB(0xf0f0f0) || detail != Alpha(core.RGB(0xf0f0f0), 0.7) {
		t.Errorf("the stack's color gave a title of %x and a description of %x", title, detail)
	}
	if got := app.Find("text:Saved").TextStyle.Color; got != palette.Text {
		t.Errorf("a default toast's title is %x, want the palette's text", got)
	}
	if plain, custom := app.Find("plain"), app.Find("custom"); custom.Frame.Size.Height-plain.Frame.Size.Height != 24 {
		t.Fatalf("custom padding made the toast %v tall, want %v", custom.Frame.Size.Height, plain.Frame.Size.Height+24)
	}

	overridden := app.Find("overridden").ID
	newFill := core.RGB(0x416631)
	app.Post(func() {
		stackFill.Set(newFill)
		radius.Set(30)
	})
	app.Settle()
	if got := app.Find("inherited").Style.Background; got != newFill {
		t.Fatalf("stack style did not follow its signal: %x", got)
	}
	if got := app.Find("overridden"); got.Style.CornerRadius != 30 || got.Style.Background != itemBackground || got.ID != overridden {
		t.Fatal("a reactive style lost the per-toast override or rebuilt the toast")
	}
	app.Drag("id:overridden", core.Point{Y: 32}, core.Point{Y: 80})
	if s := app.FindMounted("id:overridden").Style; s.Opacity != 0 || s.Transform.Scale != 0.96 {
		t.Fatalf("the swiped toast did not enter its ending style: opacity %v, scale %v", s.Opacity, s.Transform.Scale)
	}
	app.Advance(time.Second)
	if app.Exists("overridden") || !app.Exists("inherited") {
		t.Fatal("a swipe did not dismiss only the swiped toast")
	}
}

// TestToastReplacementFollowsNewContent keeps a toast's mounted view when its
// content is replaced with the same ID, while updating what it shows and does.
func TestToastReplacementFollowsNewContent(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		firstAction, nextAction := false, false
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, TestID: "toasts", CloseLabel: "Close"}
		})

		toaster.Show(ToastContent{ID: "same", Title: "First", Description: "old", Leading: core.Text{Content: "first-leading"}, Action: "First action", Duration: -1,
			OnAction: func() { firstAction = true }})
		app.Advance(time.Second)
		oldY := toastY(app, "same")

		toaster.Show(ToastContent{ID: "same", Title: "Second", Description: "new", Leading: core.Text{Content: "second-leading"}, Action: "Second action", Duration: -1,
			OnAction: func() { nextAction = true }})
		app.Advance(time.Second)

		texts := app.Texts()
		if slices.Contains(texts, "First") || slices.Contains(texts, "old") || slices.Contains(texts, "first-leading") || slices.Contains(texts, "First action") ||
			!slices.Contains(texts, "Second") || !slices.Contains(texts, "new") || !slices.Contains(texts, "second-leading") || !slices.Contains(texts, "Second action") {
			t.Fatalf("replacement showed texts %v", texts)
		}
		if got := toastY(app, "same"); got != oldY {
			t.Fatalf("replacement moved toast from %v to %v", oldY, got)
		}
		if !slices.Contains(app.Announced, "Second. new") {
			t.Fatalf("the replacement was not read out: %q", app.Announced)
		}
		app.Press("same-action")
		if !nextAction || firstAction {
			t.Fatalf("replacement action states are first=%v next=%v", firstAction, nextAction)
		}
	})
}

// TestToastKeepsContentAcrossOtherToasts builds a toast's content once while other toasts come and go, so a leading spinner or a focused action button is not built again, and lays out a toast without a leading view or an action with no room for them.
func TestToastKeepsContentAcrossOtherToasts(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		builds := 0
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, TestID: "toasts", CloseLabel: "Close"}
		})
		toaster.Show(ToastContent{ID: "kept", Title: "Kept", Leading: countedBuild{&builds}, Action: "Undo", Duration: -1})
		toaster.Show(ToastContent{ID: "bare", Title: "Bare", Duration: -1})
		app.Advance(time.Second)
		before := builds
		toaster.Show(ToastContent{ID: "other", Title: "Other", Duration: -1})
		app.Advance(time.Second)
		toaster.Dismiss("other")
		app.Advance(time.Second)
		if builds != before {
			t.Fatalf("other toasts built a toast's leading view %d more times", builds-before)
		}
		if row := app.Find("text:Bare").Parent.Parent; len(row.Children) != 2 {
			t.Fatalf("a toast without a leading view or an action lays out %d views beside its text, want only its close button", len(row.Children)-1)
		}
	})
}

// countedBuild counts its builds.
type countedBuild struct{ n *int }

func (c countedBuild) Render(*core.Scope) core.Node {
	*c.n++
	return core.Text{Content: "counted"}
}

// TestToastFromClosingScope shows a toast from a scope that then goes,
// as a dialog's button does as the dialog closes, and still closes it
// with its close button and lets the next one time out.
func TestToastFromClosingScope(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		var dialog *core.Scope
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			dialog = cx.Child()
			return Toasts{Toaster: toaster, TestID: "toasts", CloseLabel: "Close"}
		})
		app.Post(func() {
			dialog.Post(func() {
				toaster.Show(ToastContent{ID: "accepted", Title: "Accepted"})
				dialog.Dispose()
			})
		})
		app.Advance(time.Second)
		app.Press("accepted-close")
		app.Advance(time.Second)
		if app.Exists("accepted") {
			t.Fatal("a toast shown from a scope that went did not close")
		}
		toaster.Show(ToastContent{ID: "next", Title: "Next", Duration: time.Second})
		app.Advance(2 * time.Second)
		if app.Exists("next") {
			t.Fatal("the next toast stayed past its duration")
		}
	})
}

// TestToastPlacements places the stack at each edge, swipes toward it by
// default and turns it around with Reverse.
func TestToastPlacements(t *testing.T) {
	for _, c := range []struct {
		p    ToastPlacement
		want toast.SwipeDirection
	}{{ToastTop, toast.SwipeUp}, {ToastBottomEnd, toast.SwipeDown}, {ToastStart, toast.SwipeLeft}, {ToastEnd, toast.SwipeRight}} {
		if got := ToastSwipeEdge.direction(c.p, false); got != c.want {
			t.Fatalf("placement %d swipes %v, want %v", c.p, got, c.want)
		}
	}
	if got := ToastSwipeEdge.direction(ToastStart, true); got != toast.SwipeRight {
		t.Fatalf("the start side swipes %v right to left", got)
	}
	if got := ToastSwipeOff.direction(ToastTop, false); got != toast.SwipeOff {
		t.Fatalf("ToastSwipeOff swipes %v", got)
	}
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, Placement: ToastEnd, Reverse: true, CloseLabel: "Close"}
		})
		toaster.Show(toastOf("old", ToastPlain, "Old", "", "", nil))
		app.Advance(time.Second)
		toaster.Show(toastOf("new", ToastPlain, "New", "", "", nil))
		app.Advance(time.Second)
		if !(toastY(app, "old") <= toastY(app, "new")) {
			t.Fatal("Reverse left the newest toast above the oldest")
		}
	})
}

// TestToastsExpandedTakePresses opens the stack and closes an older toast
// where it shows, not only the one in front.
func TestToastsExpandedTakePresses(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, Placement: ToastBottom, TestID: "toasts", CloseLabel: "Close"}
		})
		toaster.Show(ToastContent{ID: "old", Title: "Old", Duration: -1})
		app.Advance(time.Second)
		toaster.Show(ToastContent{ID: "new", Title: "New", Duration: -1})
		app.Advance(time.Second)
		app.Hover("toasts")
		app.Advance(time.Second)
		if !(toastY(app, "old") < toastY(app, "new")) {
			t.Fatal("the expanded toasts lie one over the other")
		}
		app.Press("old-close")
		app.Advance(time.Second)
		if app.Exists("old") || !app.Exists("new") {
			t.Fatal("the close button of the older toast did not close it")
		}
	})
}

// TestToastLimit keeps Limit toasts showing, the oldest first, and tells
// a toast's OnClose when it has gone, merged into or timed out.
func TestToastLimit(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		closed := 0
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			toaster.Limit = 1
			return Toasts{Toaster: toaster, CloseLabel: "Close"}
		})
		toaster.Show(toastOf("a", ToastPlain, "A", "", "", nil))
		toaster.Show(toastOf("b", ToastPlain, "B", "", "", nil))
		app.Advance(100 * time.Millisecond)
		if !app.Exists("a") || app.Exists("b") {
			t.Fatal("Limit 1 did not show the oldest alone")
		}
		onClose := func() { closed++ }
		// An error shows over them, and a repeat of it merges into it.
		toaster.Show(ToastContent{ID: "c", Type: ToastError, Title: "C", Duration: time.Second, OnClose: onClose})
		toaster.Show(ToastContent{ID: "c", Type: ToastError, Title: "C again", Duration: time.Second, OnClose: onClose})
		if closed != 1 {
			t.Fatalf("replacing a toast closed %d", closed)
		}
		app.Advance(3 * time.Second)
		if closed != 2 {
			t.Fatalf("OnClose ran %d times once it timed out", closed)
		}
	})
}

// TestToastClosing closes the newest toast: it stays where it shows and
// takes no presses, and the others close up at once, at every line a stack
// grows from.
func TestToastClosing(t *testing.T) {
	for _, c := range []struct {
		name    string
		p       ToastPlacement
		reverse bool
	}{{"top", ToastTop, false}, {"bottom", ToastBottomEnd, false}, {"top reversed", ToastTop, true},
		{"bottom reversed", ToastBottom, true}, {"side", ToastEnd, false}, {"side reversed", ToastStart, true}} {
		t.Run(c.name, func(t *testing.T) {
			var toaster *Toaster
			app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
				toaster = NewToaster(cx)
				return Toasts{Toaster: toaster, Placement: c.p, Reverse: c.reverse, TestID: "toasts", CloseLabel: "Close"}
			})
			toaster.Show(ToastContent{ID: "old", Title: "Old", Duration: -1})
			app.Advance(time.Second)
			toaster.Show(ToastContent{ID: "new", Title: "New", Action: "Undo", Duration: -1})
			app.Advance(time.Second)
			if app.Find("new").Animated.Set != 0 {
				t.Fatal("the toast's root binds animated values, which backends draw without its Transition")
			}
			height := app.Find("toasts").Frame.Size.Height
			y := toastY(app, "new")
			toaster.Dismiss("new")
			app.Advance(16 * time.Millisecond)
			if !app.Exists("new") {
				t.Fatal("the toast left before its exit")
			}
			if got := toastY(app, "new"); got != y {
				t.Fatalf("the toast going moved from %v to %v", y, got)
			}
			if got := app.Find("toasts").Frame.Size.Height; got >= height {
				t.Fatalf("the stack is %v tall going, %v before: the toast going takes room", got, height)
			}
			settled := toastY(app, "old")
			going := app.Find("new")
			if top := going.Children[len(going.Children)-1]; top.Frame.Size.Width < going.Frame.Size.Width-2 {
				t.Fatalf("nothing covers the toast going, whose buttons would take presses: %v", top.Frame)
			}
			app.Advance(time.Second)
			if app.Exists("new") {
				t.Fatal("the toast stayed after its exit")
			}
			if got := toastY(app, "old"); got != settled {
				t.Fatalf("the older toast's place went from %v to %v once the newest had gone", settled, got)
			}
		})
	}
}

// toastY is where the stack lays a toast out, in the window.
func toastY(app *nsgotest.App, id string) core.Points {
	var y core.Points
	for v := app.Find(id).Parent; v != nil; v = v.Parent {
		y += v.Frame.Origin.Y
	}
	return y
}

// toastOf is a toast's content, short.
func toastOf(id string, kind ToastType, title, description, action string, onAction func()) ToastContent {
	return ToastContent{ID: id, Type: kind, Title: title, Description: description, Action: action, OnAction: onAction}
}

// TestToastsQueue shows toasts in turn with a limit, an error before
// those waiting, a polite status region, and timeouts paused while the
// app is in the background.
func TestToastsQueue(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		var waiting int
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			toaster.Limit = 1
			return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node { waiting = toaster.Waiting(); return core.View{} }),
				Toasts{Toaster: toaster, TestID: "toasts", Polite: true},
			}}
		})
		toaster.Show(ToastContent{ID: "first", Title: "First", Duration: 2 * time.Second})
		toaster.Show(ToastContent{ID: "second", Title: "Second", Duration: 2 * time.Second})
		app.Advance(time.Second)
		if waiting != 1 || !app.Exists("first") || app.Exists("second") {
			t.Fatalf("waiting %d, want the first showing and one behind it\n%s", waiting, app.Dump())
		}
		if v := app.Find("first"); v.Props.Role != core.RoleStatus {
			t.Errorf("a polite toast reads %v, want a status region", v.Props.Role)
		}
		// An error shows before the toasts waiting.
		toaster.Show(ToastContent{ID: "bad", Type: ToastError, Title: "Bad", Duration: 2 * time.Second})
		app.Advance(time.Second)
		if waiting != 2 || !app.Exists("bad") {
			t.Fatalf("waiting %d, want the error showing and two behind it\n%s", waiting, app.Dump())
		}
		if v := app.Find("bad"); v.Props.Role != core.RoleAlert {
			t.Errorf("an error reads %v, want an alert", v.Props.Role)
		}
		// A window in the background holds them.
		app.SetEnvironment(core.Environment{Lifecycle: core.LifecycleBackground})
		app.Advance(5 * time.Second)
		if !app.Exists("bad") {
			t.Fatal("a toast timed out while the app was in the background")
		}
		// Returning to the foreground resumes the timeout and lets it expire.
		app.SetEnvironment(core.Environment{Lifecycle: core.LifecycleActive})
		app.Advance(3 * time.Second)
		if app.Exists("bad") || !app.Exists("first") {
			t.Fatalf("after returning to foreground: bad=%v first=%v\n%s", app.Exists("bad"), app.Exists("first"), app.Dump())
		}
		// A visible but unfocused window (LifecycleInactive) keeps counting down.
		app.SetEnvironment(core.Environment{Lifecycle: core.LifecycleInactive})
		toaster.Show(ToastContent{ID: "inactive", Type: ToastError, Title: "Inactive", Duration: 2 * time.Second})
		app.Advance(time.Second)
		if !app.Exists("inactive") {
			t.Fatal("toast disappeared early while inactive")
		}
		app.Advance(2 * time.Second)
		if app.Exists("inactive") {
			t.Fatal("a toast paused while the app was inactive instead of only in the background")
		}
	})
}

func TestToastsMotionAndReduceMotion(t *testing.T) {
	var toaster *Toaster
	custom := toast.Motion{Duration: 800 * time.Millisecond, ExitDuration: 600 * time.Millisecond}
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		toaster = NewToaster(cx)
		return Toasts{Toaster: toaster, Motion: custom, CloseLabel: "Close"}
	})
	toaster.Show(ToastContent{ID: "slow", Title: "Slow", Duration: -1})
	app.Advance(time.Second)
	if got := app.Find("slow").Style.Transition.Duration; got != custom.Duration {
		t.Fatalf("enter transition = %v, want %v", got, custom.Duration)
	}
	toaster.Dismiss("slow")
	app.Advance(300 * time.Millisecond)
	if !app.Exists("slow") {
		t.Fatal("toast left before custom ExitDuration (600ms)")
	}
	if got := app.FindMounted("id:slow").Style.Transition.Duration; got != custom.ExitDuration {
		t.Fatalf("exit transition = %v, want %v", got, custom.ExitDuration)
	}
	app.Advance(400 * time.Millisecond)
	if app.Exists("slow") {
		t.Fatal("toast stayed after custom ExitDuration")
	}

	// Runtime ReduceMotion switches transitions to 0 and skips exit wait.
	toaster.Show(ToastContent{ID: "live", Title: "Live", Duration: -1})
	app.Advance(time.Second)
	app.SetEnvironment(core.Environment{ReduceMotion: true})
	if got := app.Find("id:live").Style.Transition.Duration; got != 0 {
		t.Fatalf("transition under ReduceMotion = %v, want 0", got)
	}
	toaster.Dismiss("live")
	app.Advance(100 * time.Millisecond)
	if app.Exists("id:live") {
		t.Fatal("toast waited for ExitDuration under ReduceMotion")
	}

	// Turning ReduceMotion back off restores the configured motion.
	app.SetEnvironment(core.Environment{ReduceMotion: false})
	toaster.Show(ToastContent{ID: "back", Title: "Back", Duration: -1})
	app.Advance(time.Second)
	if got := app.Find("back").Style.Transition.Duration; got != custom.Duration {
		t.Fatalf("transition after leaving ReduceMotion = %v, want %v", got, custom.Duration)
	}
}

func TestToastsDismissWaitingNeverShowsOrAnnounces(t *testing.T) {
	var toaster *Toaster
	var waiting int
	firstClosed, secondClosed, thirdClosed := 0, 0, 0
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		toaster = NewToaster(cx)
		toaster.Limit = 1
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node { waiting = toaster.Waiting(); return core.View{} }),
			Toasts{Toaster: toaster, TestID: "toasts"},
		}}
	})
	toaster.Show(ToastContent{ID: "first", Title: "First", Duration: -1, Priority: toast.PriorityNormal, OnClose: func() { firstClosed++ }})
	toaster.Show(ToastContent{ID: "second", Title: "Second", Duration: -1, Priority: toast.PriorityNormal, OnClose: func() { secondClosed++ }})
	toaster.Show(ToastContent{ID: "third", Title: "Third", Duration: -1, Priority: toast.PriorityNormal, OnClose: func() { thirdClosed++ }})
	app.Advance(100 * time.Millisecond)
	if waiting != 2 || !app.Exists("first") || app.Exists("second") || app.Exists("third") {
		t.Fatalf("initial state: waiting=%d first=%v second=%v third=%v", waiting, app.Exists("first"), app.Exists("second"), app.Exists("third"))
	}

	toaster.Dismiss("second")
	if app.Exists("second") || waiting != 1 {
		t.Fatalf("after dismissing waiting toast: exists=%v waiting=%d", app.Exists("second"), waiting)
	}
	app.Advance(100 * time.Millisecond)
	if secondClosed != 1 {
		t.Fatalf("second OnClose = %d, want 1", secondClosed)
	}
	if slices.Contains(app.Announced, "Second") {
		t.Fatalf("dismissed waiting toast was announced: %v", app.Announced)
	}

	toaster.DismissAll()
	if app.Exists("third") || waiting != 0 {
		t.Fatalf("after DismissAll: third=%v waiting=%d", app.Exists("third"), waiting)
	}
	app.Advance(time.Second)
	if firstClosed != 1 || thirdClosed != 1 {
		t.Fatalf("OnClose counts: first=%d third=%d, want 1 each", firstClosed, thirdClosed)
	}
	if slices.Contains(app.Announced, "Third") {
		t.Fatalf("waiting toast dismissed by DismissAll was announced: %v", app.Announced)
	}
}

func TestToastsPriorityOptOutAndPoliteMergeAndLimitChange(t *testing.T) {
	var toaster *Toaster
	var waiting int
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		toaster = NewToaster(cx)
		toaster.Limit = 1
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node { waiting = toaster.Waiting(); return core.View{} }),
			Toasts{Toaster: toaster, TestID: "toasts", Polite: true},
		}}
	})

	// Explicit PriorityNormal on a sticky toast and an error keeps them in turn.
	toaster.Show(ToastContent{ID: "job", Type: ToastInfo, Title: "Uploading", Duration: -1, Priority: toast.PriorityNormal})
	toaster.Show(ToastContent{ID: "minor", Type: ToastError, Title: "Minor error", Duration: -1, Priority: toast.PriorityNormal})
	app.Advance(time.Second)
	if !app.Exists("job") || app.Exists("minor") || waiting != 1 {
		t.Fatalf("explicit PriorityNormal error preempted queue: job=%v minor=%v waiting=%d", app.Exists("job"), app.Exists("minor"), waiting)
	}

	// Polite info toast is RoleStatus with LiveRegionPolite and is not double-announced.
	if v := app.Find("job"); v.Props.Role != core.RoleStatus || v.Props.LiveRegion != core.LiveRegionPolite {
		t.Fatalf("polite info toast: role=%v live=%v, want status+polite", v.Props.Role, v.Props.LiveRegion)
	}
	if slices.Contains(app.Announced, "Uploading") {
		t.Fatalf("polite status toast also triggered Announce: %v", app.Announced)
	}

	// Merging ToastInfo -> ToastError in place switches RoleStatus -> RoleAlert and announces the error.
	toaster.Show(ToastContent{ID: "job", Type: ToastError, Title: "Upload failed", Duration: -1, Priority: toast.PriorityNormal})
	app.Advance(100 * time.Millisecond)
	if v := app.Find("job"); v.Props.Role != core.RoleAlert || v.Props.LiveRegion != core.LiveRegionNone {
		t.Fatalf("merged error toast: role=%v live=%v, want alert+none", v.Props.Role, v.Props.LiveRegion)
	}
	if !slices.Contains(app.Announced, "Upload failed") {
		t.Fatalf("merged error toast was not announced: %v", app.Announced)
	}

	// Changing Toaster.Limit at runtime updates shown and waiting.
	toaster.Limit = 2
	app.Advance(100 * time.Millisecond)
	if !app.Exists("job") || !app.Exists("minor") || waiting != 0 {
		t.Fatalf("after raising Toaster.Limit: job=%v minor=%v waiting=%d", app.Exists("job"), app.Exists("minor"), waiting)
	}
	toaster.Limit = 1
	app.Advance(100 * time.Millisecond)
	if !app.Exists("job") || app.Exists("minor") || waiting != 1 {
		t.Fatalf("after lowering Toaster.Limit: job=%v minor=%v waiting=%d", app.Exists("job"), app.Exists("minor"), waiting)
	}
}
