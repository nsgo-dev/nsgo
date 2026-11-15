package nsgotest

import (
	"slices"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Overlay is an overlay over other apps the app opened (mobile.OpenOverlay), as the test's platform shows it: over the surface, which stands for the screen. Its content is a tree of its own that steps do not reach.
type Overlay struct {
	// Frame is where it shows and how large, in the surface's points, kept within the surface as Android keeps an overlay on the screen.
	Frame driver.WindowFrame
	// Visible is whether it shows; Focusable, Passive and Accessible are its input modes.
	Visible                        bool
	Focusable, Passive, Accessible bool
	// Closed is whether it closed, and Ended why the platform closed it itself, 0 when the app did.
	Closed bool
	Ended  core.OverlayEnd
	tree   *core.Tree
	// env is the environment the test's platform reports it, the app's as it opened.
	env driver.Environment
}

// RevokeOverlayPermission takes the app's permission to show overlays away, as the user does in the system's settings: an overlay open ends, OverlayEndRevoked, and settles.
func (a *App) RevokeOverlayPermission() {
	a.t.Helper()
	a.OverlayPermission = core.OverlayDenied
	if o := a.Overlay; o != nil && !o.Closed {
		a.end(o, core.OverlayEndRevoked)
	}
	a.Settle()
}

// openOverlay shows the overlay cfg opens, or ends it at once without the permission.
func (a *App) openOverlay(cfg *driver.SurfaceConfig) {
	i := slices.IndexFunc(a.tree.Surfaces(), func(t *core.Tree) bool { return t.ID() == cfg.ID })
	if i < 0 {
		return
	}
	o := &Overlay{Visible: !cfg.Hidden, Focusable: cfg.Focusable, Passive: cfg.Passive, Accessible: cfg.Accessible,
		tree: a.tree.Surfaces()[i]}
	if a.env != nil {
		o.env = *a.env
	}
	a.Overlay = o
	switch a.OverlayPermission {
	case core.OverlayGranted:
		a.place(o, cfg.Frame)
	case core.OverlayUnavailable:
		a.end(o, core.OverlayEndUnavailable)
	default:
		a.end(o, core.OverlayEndDenied)
	}
}

// place applies frame to o within the surface and reports the frame applied in o's environment.
func (a *App) place(o *Overlay, frame driver.WindowFrame) {
	o.Frame = frame.Within(driver.Bounds{Size: a.size})
	o.Frame.Move = true
	o.env.WindowFrame = o.Frame
	o.tree.Resize(driver.Size{Width: o.Frame.Width, Height: o.Frame.Height})
	o.tree.SetEnvironment(o.env)
}

// end has the platform end o for reason.
func (a *App) end(o *Overlay, reason core.OverlayEnd) {
	o.Ended = reason
	t := o.tree
	a.Post(func() { t.Dispatch(driver.Event{Kind: driver.EventWindowEnded, Index: int(reason)}) })
}

// commitOverlay commits the open overlay's tree and applies its window commands, and reports whether it sent any; one that closed is gone.
func (a *App) commitOverlay() bool {
	o := a.Overlay
	if o == nil || o.tree == nil {
		return false
	}
	if o.tree.Closed() {
		o.Closed, o.Visible = true, false
		o.tree.Forget()
		o.tree = nil
		return true
	}
	b := o.tree.Commit()
	if b == nil {
		return false
	}
	for _, it := range b.Items {
		if it.Kind != driver.ItemWindow {
			continue
		}
		switch it.Window.Action {
		case driver.WindowSetFrame:
			f := it.Window.Frame
			if f.Width <= 0 || f.Height <= 0 {
				f.Width, f.Height = o.Frame.Width, o.Frame.Height
			}
			if !f.Move {
				f.X, f.Y, f.Move = o.Frame.X, o.Frame.Y, true
			}
			a.place(o, f)
		case driver.WindowShow, driver.WindowOrderFront:
			o.Visible = true
		case driver.WindowHide:
			o.Visible = false
		}
	}
	return true
}
