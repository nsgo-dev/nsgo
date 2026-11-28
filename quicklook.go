package nsgo

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/backend"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// QuickLookPreview builds the root element previewing the file at path in a
// macOS QuickLook Preview Extension.
type QuickLookPreview func(cx *core.Scope, path string) core.Node

// RunQuickLook runs a macOS QuickLook Preview Extension
// (com.apple.quicklook.preview), registering an NSViewController conforming to
// QLPreviewingController that mounts an embedded NSGo view for each previewed
// file and handing control to NSExtensionMain.
//
// preview receives the previewed file's path; the view's host channel also
// provides "path" and "url" in core.Scope.HostParams. WindowSize sets the
// controller's preferred content size (640 by 480 by default). Component,
// Fonts, Appearance, DataDir and OnError apply.
//
// On platforms other than macOS, RunQuickLook returns immediately.
func RunQuickLook(preview QuickLookPreview, opts ...Option) {
	o := options{platform: backend.Options{Width: 640, Height: 480, Appearance: startAppearance()}}
	for _, fn := range opts {
		fn(&o)
	}
	h := engine.HostOptions{OnError: o.onError, DataDir: o.dataDir}
	backend.RunQuickLook(preview, o.platform, h)
}

// QuickLookThumbnail builds the root element drawing the thumbnail of the
// file at path in a macOS QuickLook Thumbnail Extension, at most size points.
type QuickLookThumbnail func(cx *core.Scope, path string, size core.Size) core.Node

// RunQuickLookThumbnail runs a macOS QuickLook Thumbnail Extension
// (com.apple.quicklook.thumbnail), registering a QLThumbnailProvider that
// mounts thumbnail for each file Finder asks the icon of in a window that
// never shows, draws its views off the screen at the request's largest size
// and scale, and replies with the drawing, and handing control to
// NSExtensionMain.
//
// thumbnail receives the file's path and the largest size; the view's host
// channel also provides "path", "url", "width", "height" and "scale" in
// core.Scope.HostParams. Requests arrive at once from several threads: up to
// four render together, and one that has not drawn ten seconds after it
// came fails, as does one whose view draws nothing, and QuickLook keeps the
// file's icon. The drawing leaves out what renderInContext: cannot draw:
// GPU and shader views, video, web pages, backdrop blur and glass.
// Component, Fonts, Appearance, DataDir and OnError apply.
//
// On platforms other than macOS, RunQuickLookThumbnail returns immediately.
func RunQuickLookThumbnail(thumbnail QuickLookThumbnail, opts ...Option) {
	o := options{platform: backend.Options{Appearance: startAppearance()}}
	for _, fn := range opts {
		fn(&o)
	}
	h := engine.HostOptions{OnError: o.onError, DataDir: o.dataDir}
	backend.RunQuickLookThumbnail(thumbnail, o.platform, h)
}
