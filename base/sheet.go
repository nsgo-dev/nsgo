package base

import (
	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/base/internal/sheet"
	"github.com/nsgo-dev/nsgo/core"
)

// Sheet is a panel at an edge of the app (Side) over a backdrop that
// covers the rest and takes its presses, on top of everything in its root
// view, the same on every device. It slides in from its edge as the
// backdrop fades in, and back out as it closes, at once when the user
// asked for less motion; OnOpen and OnClose follow the motion's end. A
// press on the backdrop, Escape and back close it, and with
// CloseOnSwipe a bottom sheet follows a downward drag and closes on a
// release past 30% of its height or 800 points a second.
type Sheet struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()
	// OnConfirm is the default action, as Dialog's: Return runs it while the sheet is open, then asks to close with ReasonSelect.
	OnConfirm func()

	// Side is the edge the panel sits on; the bottom by default.
	Side                   core.Placement
	NoCloseOnEscape        bool
	NoCloseOnBackdropPress bool
	CloseOnSwipe           bool
	// BackdropStyle styles the backdrop, which fades with the panel: give
	// it a translucent background.
	BackdropClass     string
	BackdropStyle     core.Style
	BackdropStyleFunc func() core.Style
	PopupClass        string
	PopupStyle        core.Style
	PopupStyleFunc    func() core.Style
	// StartingStyle and EndingStyle style the panel while it enters and leaves;
	// include a Transition in the style for custom timing.
	StartingClass string
	StartingStyle core.Style
	EndingClass   string
	EndingStyle   core.Style
	// Class and Style style the layer that holds the backdrop and the
	// panel, which covers the root view.
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	// Children are the content, made on each opening.
	Children []core.Node
	// Label names the sheet for screen readers, a dialog, and AccessibilityDescription is read after it.
	Label                        string
	LabelFunc                    func() string
	AccessibilityDescription     string
	AccessibilityDescriptionFunc func() string
	// Role is what screen readers announce it as, RoleDialog when 0 or RoleAlertDialog for one asking for an answer, as Dialog's.
	Role   core.Role
	TestID string
}

// Render implements core.Node.
func (s Sheet) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, s.Open, s.DefaultOpen, s.OnOpenChange, s.OnOpen, s.OnClose)
	st.onCancel = s.OnCancel
	role := s.Role
	if role == core.RoleNone {
		role = core.RoleDialog
	}
	side := s.Side
	if side == core.PlaceDefault {
		side = core.PlaceBottom
	}
	m := sheet.NewMotion(cx, st.open, side)
	st.lifecycle(cx, m.There)
	return sheet.Layer{Motion: m, Side: side, Starting: parts.Of(s.StartingClass, s.StartingStyle, nil), Ending: parts.Of(s.EndingClass, s.EndingStyle, nil), Backdrop: parts.Of(s.BackdropClass, s.BackdropStyle, s.BackdropStyleFunc),
		BackdropPress: func() { st.cancel(ReasonBackdropPress, s.NoCloseOnBackdropPress) },
		Panel:         parts.Of(s.PopupClass, s.PopupStyle, s.PopupStyleFunc),
		Swipe:         s.CloseOnSwipe && side == core.PlaceBottom,
		Swiped:        func() { st.cancel(ReasonSwipe, false) },
		Cancel:        func() { st.cancel(ReasonCancel, s.NoCloseOnEscape) },
		Confirm:       st.confirm(s.OnConfirm),
		Role:          role, Label: s.Label, LabelFunc: s.LabelFunc, AccessibilityDescription: s.AccessibilityDescription,
		AccessibilityDescriptionFunc: s.AccessibilityDescriptionFunc, TestID: s.TestID, Content: s.Children,
		Layer: parts.Of(s.Class, s.Style, s.StyleFunc)}.Node()
}
