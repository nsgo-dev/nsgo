//go:build js && wasm && nsgoharmony

package backend

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/harmonyos"
)

// run fills the window with app, drawn by ArkUI: the engine runs in
// WebAssembly in the host's Web view, and the ArkTS render layer draws
// what it mounts (docs/architecture/harmonyos.md).
func run(root engine.App, app func(*core.Scope), o Options) {
	harmonyos.Run(rootOf(root, app), harmonyos.Options{ID: id, Appearance: o.Appearance, Perf: o.Monitor})
}
