package core

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// newOverlays runs app on a platform that opens no windows, as Android's: the first tree is the app's view, and the surfaces it opens commit on their own.
func newOverlays(t *testing.T, os OS, app func(cx *Scope) Node) *windows {
	w := &windows{t: t, items: map[uint32][]driver.MountItem{}}
	w.root = NewRoot(RootOptions{Post: func(func()) {}, Clock: NewManualClock(time.Unix(0, 0))})
	w.app = NewTree(w.root, ApproxTextEngine{})
	w.app.SetEnvironment(Environment{OS: os})
	w.app.Mount(app)
	w.commit()
	return w
}

// denyOverlays is an Access that grants everything but AccessOverlay.
type denyOverlays struct{}

func (denyOverlays) Allows(req AccessRequest) bool { return req.Permission != AccessOverlay }
func (denyOverlays) WebHosts() []string            { return nil }

func TestOverlayPermission(t *testing.T) {
	var cx *Scope
	w := newOverlays(t, OSIOS, func(c *Scope) Node { cx = c; return View{} })
	got := OverlayGranted
	cx.OverlayPermission(func(p OverlayPermission) { got = p })
	w.commit()
	if got != OverlayUnavailable || len(w.take(0, driver.ItemOverlayPermission)) != 0 {
		t.Fatalf("off Android: %v", got)
	}

	w = newOverlays(t, OSAndroid, func(c *Scope) Node { cx = c; return View{} })
	cx.RequestOverlayPermission(func(p OverlayPermission) { got = p })
	w.commit()
	asked := w.take(0, driver.ItemOverlayPermission)
	if len(asked) != 1 || !asked[0].On {
		t.Fatalf("asked %+v", asked)
	}
	w.dispatch(0, driver.Event{Kind: driver.EventOverlayPermission, Count: asked[0].Count, Index: driver.OverlayGranted})
	if got != OverlayGranted {
		t.Fatalf("answered %v", got)
	}

	var denied *Window
	w = newOverlays(t, OSAndroid, func(c *Scope) Node {
		c.ProvideAccess(denyOverlays{})
		c.OverlayPermission(func(p OverlayPermission) { got = p })
		denied = c.OpenOverlay(OverlayOptions{}, nil)
		return View{}
	})
	if got != OverlayUnavailable || denied != nil || len(w.take(0, driver.ItemOverlayPermission)) != 0 {
		t.Errorf("under an Access without overlays: %v, %v", got, denied)
	}
}

func TestOverlaySurface(t *testing.T) {
	var cx *Scope
	var overlay *Window
	w := newOverlays(t, OSAndroid, func(c *Scope) Node {
		cx = c
		overlay = c.OpenOverlay(OverlayOptions{Frame: WindowFrame{X: 12, Y: 18, Move: true, Width: 240, Height: 120},
			Focusable: true, Accessible: true}, func(*Scope) Node { return Text{Content: "floating"} })
		return View{}
	})
	opened := w.take(0, driver.ItemOpenSurface)
	if len(opened) != 1 || opened[0].Surface.Kind != driver.SurfaceOverlay {
		t.Fatalf("opened %+v", opened)
	}
	cfg := opened[0].Surface
	if cfg.Width != 240 || cfg.Height != 120 || cfg.Frame != (WindowFrame{X: 12, Y: 18, Move: true, Width: 240, Height: 120}) ||
		!cfg.Focusable || cfg.Passive || !cfg.Accessible || cfg.Hidden {
		t.Fatalf("config %+v", cfg)
	}
	if texts := w.texts(cfg.ID); len(texts) != 1 || texts[0] != "floating" {
		t.Fatalf("its tree shows %q", texts)
	}
	if again := cx.OpenOverlay(OverlayOptions{}, nil); again != nil {
		t.Error("a second overlay opened while one is open")
	}

	// Moving, hiding and showing are the window commands every surface takes.
	inside := w.app.Surfaces()[0]
	inside.root.SetWindowFrame(WindowFrame{X: 40, Y: 60, Move: true})
	overlay.Hide()
	overlay.Show()
	w.commit()
	var actions []driver.WindowAction
	for _, it := range w.take(cfg.ID, driver.ItemWindow) {
		actions = append(actions, it.Window.Action)
	}
	if len(actions) != 3 || actions[0] != driver.WindowSetFrame || actions[1] != driver.WindowHide || actions[2] != driver.WindowShow {
		t.Fatalf("window commands %v", actions)
	}

	overlay.Close()
	w.commit()
	if len(w.take(cfg.ID, driver.ItemWindow)) != 1 || !overlay.Closed() {
		t.Fatal("closing it sent no destroy")
	}
	if next := cx.OpenOverlay(OverlayOptions{Passive: true, Focusable: true}, nil); next == nil {
		t.Fatal("no overlay opens after the last closed")
	}
	w.commit()
	if cfg := w.take(0, driver.ItemOpenSurface)[0].Surface; !cfg.Passive || cfg.Focusable || cfg.Width != 320 || cfg.Height != 180 || cfg.Frame.Move {
		t.Errorf("a passive overlay %+v", cfg)
	}
}

func TestOverlayEnds(t *testing.T) {
	var ends []OverlayEnd
	var overlay *Window
	w := newOverlays(t, OSAndroid, func(c *Scope) Node {
		overlay = c.OpenOverlay(OverlayOptions{OnEnd: func(e OverlayEnd) { ends = append(ends, e) }}, nil)
		return View{}
	})
	id := w.take(0, driver.ItemOpenSurface)[0].Surface.ID
	w.dispatch(id, driver.Event{Kind: driver.EventWindowEnded, Index: driver.OverlayEndRevoked})
	if !overlay.Closed() || len(ends) != 1 || ends[0] != OverlayEndRevoked || ends[0].String() != "revoked" {
		t.Fatalf("revoked: closed %v, ends %v", overlay.Closed(), ends)
	}
	if len(w.take(id, driver.ItemWindow)) != 1 {
		t.Error("the platform heard no destroy")
	}
	if len(w.app.Surfaces()) != 0 {
		t.Error("the engine kept the ended tree")
	}

	// The app closing it, or its owner going, is no end of the platform's.
	ends = nil
	var owner *Scope
	var shown *Signal[bool]
	w = newOverlays(t, OSAndroid, func(c *Scope) Node {
		shown = NewSignal(c, true)
		return Show{When: shown.Get, Then: func(cx *Scope) Node {
			owner = cx
			overlay = cx.OpenOverlay(OverlayOptions{OnEnd: func(e OverlayEnd) { ends = append(ends, e) }}, nil)
			return View{}
		}}
	})
	if overlay == nil || owner == nil {
		t.Fatal("no overlay opened")
	}
	shown.Set(false)
	w.commit()
	if !overlay.Closed() || len(ends) != 0 {
		t.Fatalf("its owner went: closed %v, ends %v", overlay.Closed(), ends)
	}
}

func TestOverlayLifecycle(t *testing.T) {
	var app, inside *Env
	var overlay *Window
	w := newOverlays(t, OSAndroid, func(c *Scope) Node {
		app = c.Env()
		overlay = c.OpenOverlay(OverlayOptions{Hidden: true}, func(cx *Scope) Node {
			inside = cx.Env()
			return View{}
		})
		return View{}
	})
	id := w.take(0, driver.ItemOpenSurface)[0].Surface.ID
	if overlay.Visible() || inside == nil {
		t.Fatalf("a hidden overlay: visible %v, its tree %v", overlay.Visible(), inside)
	}

	// The overlay shows and hides through Visible, and the platform hiding it tells it too.
	overlay.Show()
	w.commit()
	if !overlay.Visible() {
		t.Fatal("shown, it is not visible")
	}
	w.dispatch(id, driver.Event{Kind: driver.EventWindowHidden})
	if overlay.Visible() {
		t.Fatal("hidden by the platform, it is still visible")
	}

	// Its tree's lifecycle is its window's, apart from the app's view's.
	tree := w.app.Surfaces()[0]
	tree.SetEnvironment(Environment{OS: OSAndroid, Lifecycle: LifecycleInactive})
	w.commit()
	if inside.Lifecycle() != LifecycleInactive || app.Lifecycle() != LifecycleActive {
		t.Fatalf("without the focus: overlay %v, app %v", inside.Lifecycle(), app.Lifecycle())
	}
	tree.SetEnvironment(Environment{OS: OSAndroid, Lifecycle: LifecycleActive})
	w.commit()
	if inside.Lifecycle() != LifecycleActive {
		t.Fatalf("with the focus: %v", inside.Lifecycle())
	}
}
