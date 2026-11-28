//go:build darwin && !ios && !nsgotui && !nsgoandroidhost

package backend

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/macos"
)

// RunQuickLook runs a macOS QuickLook Preview Extension hosting preview.
func RunQuickLook(preview func(cx *core.Scope, path string) core.Node, o Options, h engine.HostOptions) {
	macos.RunQuickLook(func(path, url string) (engine.App, engine.HostOptions) {
		host := core.NewHost()
		host.SetParams(map[string]any{"path": path, "url": url})
		ho := h
		ho.Channel = host
		ho.Settings = &engine.HostSettings{}
		return func(cx *core.Scope) core.Node {
			return preview(cx, path)
		}, ho
	}, macos.Options{
		Title:      name,
		Width:      o.Width,
		Height:     o.Height,
		ID:         id,
		Components: o.Components,
		Fonts:      o.Fonts,
		Appearance: o.Appearance,
	})
}

// RunQuickLookThumbnail runs a macOS QuickLook Thumbnail Extension drawing
// thumbnail.
func RunQuickLookThumbnail(thumbnail func(cx *core.Scope, path string, size core.Size) core.Node, o Options, h engine.HostOptions) {
	macos.RunQuickLookThumbnail(func(req macos.QuickLookThumbnailRequest) (engine.App, engine.HostOptions) {
		size := req.Size
		host := core.NewHost()
		host.SetParams(map[string]any{"path": req.Path, "url": req.URL, "width": float64(size.Width), "height": float64(size.Height), "scale": req.Scale})
		ho := h
		ho.Channel = host
		ho.Settings = &engine.HostSettings{}
		return func(cx *core.Scope) core.Node {
			return thumbnail(cx, req.Path, size)
		}, ho
	}, macos.Options{
		Title:      name,
		ID:         id,
		Components: o.Components,
		Fonts:      o.Fonts,
		Appearance: o.Appearance,
	})
}
