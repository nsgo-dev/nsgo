//go:build !nsgodev

package dev

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// Wrap is app: only nsgo dev's builds connect to its server.
func Wrap(app engine.App) engine.App { return app }

// WrapApp is app: only nsgo dev's builds connect to its server.
func WrapApp(app func(*core.Scope)) func(*core.Scope) { return app }

// Start does nothing: only nsgo dev's builds trace and serve DevTools.
func Start() {}

// Build is whether nsgo dev built the app.
const Build = false
