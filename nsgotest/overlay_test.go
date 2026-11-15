package nsgotest

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// overlayApp opens an overlay as it starts, keeping the overlay, the scope of its content and the ends it hears.
type overlayApp struct {
	overlay *core.Window
	inside  *core.Scope
	ends    []core.OverlayEnd
}

func (o *overlayApp) root(opts core.OverlayOptions) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		opts.OnEnd = func(e core.OverlayEnd) { o.ends = append(o.ends, e) }
		o.overlay = cx.OpenOverlay(opts, func(cx *core.Scope) core.Node {
			o.inside = cx
			return core.Text{Content: "floating"}
		})
		return core.View{}
	}
}

var android = Environment(driver.Environment{OS: driver.OSAndroid})

func TestOverlayRefusedWithoutThePermission(t *testing.T) {
	o := &overlayApp{}
	app := Start(t, o.root(core.OverlayOptions{}), android)
	if app.Overlay == nil || !app.Overlay.Closed || app.Overlay.Ended != core.OverlayEndDenied || app.Overlay.Visible {
		t.Fatalf("overlay %+v", app.Overlay)
	}
	if !o.overlay.Closed() || len(o.ends) != 1 || o.ends[0] != core.OverlayEndDenied {
		t.Fatalf("the app heard %v, closed %v", o.ends, o.overlay.Closed())
	}
}

func TestOverlayMovesHidesAndCloses(t *testing.T) {
	o := &overlayApp{}
	app := Start(t, func(cx *core.Scope) core.Node { return core.View{} }, android)
	app.OverlayPermission = core.OverlayGranted
	app.Post(func() {
		app.root.Batch(func() {
			o.root(core.OverlayOptions{Frame: core.WindowFrame{X: 300, Y: -20, Move: true, Width: 200, Height: 100}})(app.root)
		})
	})
	app.Settle()
	ov := app.Overlay
	if ov == nil || ov.Closed || !ov.Visible || ov.Frame != (driver.WindowFrame{X: 190, Y: 0, Move: true, Width: 200, Height: 100}) {
		t.Fatalf("opened %+v", ov)
	}
	var frame core.WindowFrame
	app.Post(func() { frame = o.inside.Env().WindowFrame() })
	app.Settle()
	if frame != ov.Frame {
		t.Errorf("its content sees the frame %+v, the platform applied %+v", frame, ov.Frame)
	}

	app.Post(func() { o.inside.SetWindowFrame(core.WindowFrame{X: 20, Y: 900, Move: true}) })
	app.Settle()
	if ov.Frame != (driver.WindowFrame{X: 20, Y: 744, Move: true, Width: 200, Height: 100}) {
		t.Errorf("moved to %+v", ov.Frame)
	}
	app.Post(o.overlay.Hide)
	app.Settle()
	if ov.Visible || o.overlay.Visible() {
		t.Error("hiding left it shown")
	}
	app.Post(o.overlay.Show)
	app.Settle()
	if !ov.Visible {
		t.Error("showing left it hidden")
	}
	app.Post(o.overlay.Close)
	app.Settle()
	if !ov.Closed || ov.Ended != 0 || len(o.ends) != 0 {
		t.Errorf("closed by the app: %+v, ends %v", ov, o.ends)
	}
}

func TestOverlayEndsAsThePermissionIsRevoked(t *testing.T) {
	o := &overlayApp{}
	app := Start(t, func(cx *core.Scope) core.Node { return core.View{} }, android)
	var asked core.OverlayPermission
	app.OverlayAnswer = core.OverlayGranted
	app.Post(func() { app.root.RequestOverlayPermission(func(p core.OverlayPermission) { asked = p }) })
	app.Settle()
	if asked != core.OverlayGranted || app.OverlaySettings != 1 {
		t.Fatalf("the request answered %v after %d settings", asked, app.OverlaySettings)
	}
	app.Post(func() { app.root.Batch(func() { o.root(core.OverlayOptions{Passive: true})(app.root) }) })
	app.Settle()
	if app.Overlay == nil || app.Overlay.Closed || !app.Overlay.Passive {
		t.Fatalf("opened %+v", app.Overlay)
	}
	app.RevokeOverlayPermission()
	if !app.Overlay.Closed || app.Overlay.Ended != core.OverlayEndRevoked || len(o.ends) != 1 || o.ends[0] != core.OverlayEndRevoked {
		t.Fatalf("revoked: %+v, the app heard %v", app.Overlay, o.ends)
	}
}

// TestHiddenOverlayEndsOnceAsThePermissionIsRevoked holds the end Android reports as the user revokes the grant, which comes whether or not the overlay shows and whether or not the app is in front: an overlay the app hid ends too, and the app hears it once.
func TestHiddenOverlayEndsOnceAsThePermissionIsRevoked(t *testing.T) {
	o := &overlayApp{}
	app := Start(t, func(cx *core.Scope) core.Node { return core.View{} }, android)
	app.OverlayPermission = core.OverlayGranted
	app.Post(func() { app.root.Batch(func() { o.root(core.OverlayOptions{})(app.root) }) })
	app.Settle()
	app.Post(o.overlay.Hide)
	app.Settle()
	if app.Overlay == nil || app.Overlay.Closed || app.Overlay.Visible {
		t.Fatalf("hidden %+v", app.Overlay)
	}
	app.RevokeOverlayPermission()
	app.RevokeOverlayPermission()
	if !app.Overlay.Closed || app.Overlay.Ended != core.OverlayEndRevoked || !o.overlay.Closed() || len(o.ends) != 1 || o.ends[0] != core.OverlayEndRevoked {
		t.Fatalf("revoked: %+v, closed %v, the app heard %v", app.Overlay, o.overlay.Closed(), o.ends)
	}
}
